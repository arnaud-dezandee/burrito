package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"sync"
	"time"

	configv1alpha1 "github.com/padok-team/burrito/api/v1alpha1"
	log "github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type WatchManager struct {
	client         client.Client
	restConfig     *rest.Config
	scheme         *runtime.Scheme
	layerInformer  cache.Controller
	runInformer    cache.Controller
	repoInformer   cache.Controller
	subscribers    map[string]chan<- []byte
	subscribersMux sync.RWMutex
	api            *API

	// Debounce fields
	debounceTimer *time.Timer
	debounceMutex sync.Mutex
	pendingUpdate bool
	lastSentData  []byte // Store the last sent data to compare against
}

func NewWatchManager(client client.Client, restConfig *rest.Config, scheme *runtime.Scheme, api *API) *WatchManager {
	return &WatchManager{
		client:      client,
		restConfig:  restConfig,
		scheme:      scheme,
		subscribers: make(map[string]chan<- []byte),
		api:         api,
	}
}

func (wm *WatchManager) Start(ctx context.Context) error {
	// Configure REST client for our custom resources
	restConfig := *wm.restConfig
	restConfig.GroupVersion = &configv1alpha1.GroupVersion
	restConfig.APIPath = "/apis"
	restConfig.NegotiatedSerializer = serializer.NewCodecFactory(wm.scheme).WithoutConversion()

	// Create REST client for informers
	restClient, err := rest.RESTClientFor(&restConfig)
	if err != nil {
		return fmt.Errorf("failed to create REST client: %w", err)
	}

	// Setup informers for TerraformLayers
	layerWatchlist := cache.NewListWatchFromClient(
		restClient,
		"terraformlayers",
		metav1.NamespaceAll,
		fields.Everything(),
	)

	_, layerController := cache.NewInformer(
		layerWatchlist,
		&configv1alpha1.TerraformLayer{},
		time.Second*10,
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				wm.handleLayerEvent("ADDED", obj)
			},
			UpdateFunc: func(oldObj, newObj interface{}) {
				if !reflect.DeepEqual(oldObj, newObj) {
					wm.handleLayerEvent("MODIFIED", newObj)
				}
			},
			DeleteFunc: func(obj interface{}) {
				wm.handleLayerEvent("DELETED", obj)
			},
		},
	)

	wm.layerInformer = layerController

	// Setup informers for TerraformRuns
	runWatchlist := cache.NewListWatchFromClient(
		restClient,
		"terraformruns",
		metav1.NamespaceAll,
		fields.Everything(),
	)

	_, runController := cache.NewInformer(
		runWatchlist,
		&configv1alpha1.TerraformRun{},
		time.Second*10,
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				wm.handleRunEvent("ADDED", obj)
			},
			UpdateFunc: func(oldObj, newObj interface{}) {
				if !reflect.DeepEqual(oldObj, newObj) {
					wm.handleLayerEvent("MODIFIED", newObj)
				}
			},
			DeleteFunc: func(obj interface{}) {
				wm.handleRunEvent("DELETED", obj)
			},
		},
	)

	wm.runInformer = runController

	// Setup informers for TerraformRepositories
	repoWatchlist := cache.NewListWatchFromClient(
		restClient,
		"terraformrepositories",
		metav1.NamespaceAll,
		fields.Everything(),
	)

	_, repoController := cache.NewInformer(
		repoWatchlist,
		&configv1alpha1.TerraformRepository{},
		time.Second*10,
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				wm.handleRepoEvent("ADDED", obj)
			},
			UpdateFunc: func(oldObj, newObj interface{}) {
				if !reflect.DeepEqual(oldObj, newObj) {
					wm.handleLayerEvent("MODIFIED", newObj)
				}
			},
			DeleteFunc: func(obj interface{}) {
				wm.handleRepoEvent("DELETED", obj)
			},
		},
	)

	wm.repoInformer = repoController

	// Start all informers
	go layerController.Run(ctx.Done())
	go runController.Run(ctx.Done())
	go repoController.Run(ctx.Done())

	// Wait for initial sync
	if !cache.WaitForCacheSync(ctx.Done(), layerController.HasSynced, runController.HasSynced, repoController.HasSynced) {
		return fmt.Errorf("failed to sync informers")
	}

	log.Info("WatchManager started successfully")
	return nil
}

func (wm *WatchManager) Subscribe(id string, ch chan<- []byte) {
	wm.subscribersMux.Lock()
	defer wm.subscribersMux.Unlock()
	wm.subscribers[id] = ch
	log.Infof("Client %s subscribed to events", id)
}

func (wm *WatchManager) Unsubscribe(id string) {
	wm.subscribersMux.Lock()
	defer wm.subscribersMux.Unlock()
	if ch, exists := wm.subscribers[id]; exists {
		close(ch)
		delete(wm.subscribers, id)
		log.Infof("Client %s unsubscribed from events", id)
	}
}

func (wm *WatchManager) broadcast(data []byte) {
	wm.subscribersMux.RLock()
	defer wm.subscribersMux.RUnlock()

	for id, ch := range wm.subscribers {
		select {
		case ch <- data:
		default:
			// Channel is full, client is not consuming fast enough
			log.Warnf("Client %s channel is full, skipping event", id)
		}
	}
}

func (wm *WatchManager) handleLayerEvent(eventType string, obj any) {
	wm.debouncedSendLayersUpdate()
}

func (wm *WatchManager) handleRunEvent(eventType string, obj any) {
	wm.debouncedSendLayersUpdate()
}

func (wm *WatchManager) handleRepoEvent(eventType string, obj any) {
	wm.debouncedSendLayersUpdate()
}

// debouncedSendLayersUpdate ensures sendLayersUpdate is called at most once every 200ms
func (wm *WatchManager) debouncedSendLayersUpdate() {
	wm.debounceMutex.Lock()
	defer wm.debounceMutex.Unlock()

	// Mark that we have a pending update
	wm.pendingUpdate = true

	// If timer is already running, reset it
	if wm.debounceTimer != nil {
		wm.debounceTimer.Stop()
	}

	// Start new timer
	wm.debounceTimer = time.AfterFunc(200*time.Millisecond, func() {
		wm.debounceMutex.Lock()
		defer wm.debounceMutex.Unlock()

		// Only send update if we still have a pending update
		if wm.pendingUpdate {
			wm.pendingUpdate = false
			wm.sendLayersUpdate()
		}
	})
}

func (wm *WatchManager) sendLayersUpdate() {
	// Get current layers data
	layersData, runs, repositories, err := wm.api.getLayersAndRuns()
	if err != nil {
		log.Errorf("failed to get layers data: %s", err)
		return
	}

	results := wm.api.buildLayersResponse(layersData, runs, repositories)

	data, err := json.Marshal(&layersResponse{Results: results})
	if err != nil {
		log.Errorf("failed to marshal layers data: %s", err)
		return
	}

	// Compare with last sent data to avoid sending identical updates
	if wm.lastSentData != nil && bytes.Equal(data, wm.lastSentData) {
		log.Debug("Skipping identical layers update")
		return
	}

	// Store the current data as last sent
	wm.lastSentData = make([]byte, len(data))
	copy(wm.lastSentData, data)

	log.Debug("Broadcasting layers update to subscribers")
	wm.broadcast(data)
}
