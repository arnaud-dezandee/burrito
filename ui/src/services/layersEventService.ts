import { Observable, BehaviorSubject, fromEventPattern, EMPTY } from 'rxjs';
import { map, catchError, share } from 'rxjs/operators';
import { Layers } from '@/clients/layers/types';

export class LayersEventService {
  private eventSource: EventSource | null = null;
  private layersSubject = new BehaviorSubject<Layers | null>(null);
  private connectionStatusSubject = new BehaviorSubject<
    'connecting' | 'connected' | 'disconnected' | 'error'
  >('disconnected');

  public layers$ = this.layersSubject.asObservable();
  public connectionStatus$ = this.connectionStatusSubject.asObservable();

  constructor(private baseUrl: string) {}

  connect(): Observable<Layers> {
    if (this.eventSource) {
      this.disconnect();
    }

    this.connectionStatusSubject.next('connecting');

    // Create the EventSource
    this.eventSource = new EventSource(`${this.baseUrl}/layers/events`);

    // Create observable from EventSource events
    const eventObservable = fromEventPattern<MessageEvent>(
      (handler) => {
        if (this.eventSource) {
          this.eventSource.addEventListener('message', handler);
          this.eventSource.addEventListener('open', () => {
            this.connectionStatusSubject.next('connected');
          });
          this.eventSource.addEventListener('error', () => {
            this.connectionStatusSubject.next('error');
          });
        }
      },
      (handler) => {
        if (this.eventSource) {
          this.eventSource.removeEventListener('message', handler);
        }
      }
    ).pipe(
      map((event: MessageEvent) => {
        try {
          const data = JSON.parse(event.data) as Layers;
          this.layersSubject.next(data);
          return data;
        } catch (error) {
          console.error('Failed to parse SSE data:', error);
          throw error;
        }
      }),
      catchError((error) => {
        console.error('EventSource error:', error);
        this.connectionStatusSubject.next('error');
        return EMPTY;
      }),
      share()
    );

    return eventObservable;
  }

  disconnect(): void {
    if (this.eventSource) {
      this.eventSource.close();
      this.eventSource = null;
      this.connectionStatusSubject.next('disconnected');
    }
  }

  getCurrentLayers(): Layers | null {
    return this.layersSubject.value;
  }

  getConnectionStatus(): 'connecting' | 'connected' | 'disconnected' | 'error' {
    return this.connectionStatusSubject.value;
  }
}

// Export a singleton instance
export const layersEventService = new LayersEventService(
  import.meta.env.VITE_API_BASE_URL || ''
);
