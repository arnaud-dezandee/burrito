import { useEffect, useState } from 'react';
import { layersEventService } from '@/services/layersEventService';
import type { Layers } from '@/clients/layers/types';

export const useLayersEvents = () => {
  const [layers, setLayers] = useState<Layers | null>(null);
  const [connectionStatus, setConnectionStatus] = useState<
    'connecting' | 'connected' | 'disconnected' | 'error'
  >('disconnected');
  const [isLoading, setIsLoading] = useState(true);

  useEffect(() => {
    // Subscribe to connection status changes
    const statusSubscription = layersEventService.connectionStatus$.subscribe(
      (status) => {
        setConnectionStatus(status);
        if (status === 'connected') {
          setIsLoading(false);
        }
      }
    );

    // Connect to the event stream
    const layersSubscription = layersEventService.connect().subscribe({
      next: (layersData) => {
        setLayers(layersData);
        setIsLoading(false);
      },
      error: (error) => {
        console.error('Layers event stream error:', error);
        setIsLoading(false);
      }
    });

    // Cleanup on unmount
    return () => {
      if (layersSubscription) {
        layersSubscription.unsubscribe();
      }
      if (statusSubscription) {
        statusSubscription.unsubscribe();
      }
      layersEventService.disconnect();
    };
  }, []);

  return {
    layers,
    connectionStatus,
    isLoading,
    reconnect: () => {
      setIsLoading(true);
      layersEventService.connect().subscribe({
        next: (layersData) => {
          setLayers(layersData);
          setIsLoading(false);
        },
        error: (error) => {
          console.error('Layers event stream error:', error);
          setIsLoading(false);
        }
      });
    }
  };
};
