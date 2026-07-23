import { DestroyRef, inject, Injectable, signal } from '@angular/core';
import type { LocalTrack, RemoteTrack, Room } from 'livekit-client';

import type { Call, CallJoin } from '../../core/api/api.types';

@Injectable({ providedIn: 'root' })
export class CallSessionService {
  private room: Room | null = null;
  private mediaHost: HTMLElement | null = null;
  private readonly attached = new Set<HTMLMediaElement>();
  private generation = 0;

  readonly activeCall = signal<Call | null>(null);
  readonly status = signal<'idle' | 'connecting' | 'connected' | 'failed'>('idle');
  readonly microphoneEnabled = signal(true);
  readonly cameraEnabled = signal(false);
  readonly error = signal<unknown>(null);

  constructor() {
    inject(DestroyRef).onDestroy(() => this.disconnect());
  }

  async connect(grant: CallJoin, mediaHost: HTMLElement): Promise<void> {
    this.disconnect();
    const generation = ++this.generation;
    this.activeCall.set(grant.call);
    this.status.set('connecting');
    this.error.set(null);
    this.mediaHost = mediaHost;
    try {
      const { Room, RoomEvent, Track } = await import('livekit-client');
      if (generation !== this.generation) return;
      const room = new Room({ adaptiveStream: true, dynacast: true });
      this.room = room;
      room.on(RoomEvent.TrackSubscribed, (track) => {
        if (generation === this.generation && this.room === room) this.attach(track);
      });
      room.on(RoomEvent.TrackUnsubscribed, (track) => {
        if (generation === this.generation && this.room === room) this.detach(track);
      });
      room.on(RoomEvent.Disconnected, () => {
        if (generation !== this.generation || this.room !== room) return;
        this.clearMedia();
        this.status.set('idle');
        this.activeCall.set(null);
      });
      await room.connect(grant.url, grant.token);
      if (generation !== this.generation || this.room !== room) {
        void room.disconnect();
        return;
      }
      await room.localParticipant.setMicrophoneEnabled(true);
      if (generation !== this.generation || this.room !== room) {
        void room.disconnect();
        return;
      }
      this.microphoneEnabled.set(true);
      if (grant.call.kind === 'video') {
        await room.localParticipant.setCameraEnabled(true);
        if (generation !== this.generation || this.room !== room) {
          void room.disconnect();
          return;
        }
        this.cameraEnabled.set(true);
        const localCamera = room.localParticipant.getTrackPublication(Track.Source.Camera)?.track;
        if (localCamera) this.attach(localCamera, true);
      }
      if (generation === this.generation && this.room === room) this.status.set('connected');
    } catch (error) {
      if (generation !== this.generation) return;
      this.error.set(error);
      const room = this.room;
      this.room = null;
      if (room) void room.disconnect();
      this.clearMedia();
      this.status.set('failed');
      throw error;
    }
  }

  async toggleMicrophone(): Promise<void> {
    const room = this.room;
    const generation = this.generation;
    if (!room) return;
    const enabled = !this.microphoneEnabled();
    await room.localParticipant.setMicrophoneEnabled(enabled);
    if (generation !== this.generation || this.room !== room) {
      void room.disconnect();
      return;
    }
    this.microphoneEnabled.set(enabled);
  }

  async toggleCamera(): Promise<void> {
    const room = this.room;
    const generation = this.generation;
    if (!room || this.activeCall()?.kind !== 'video') return;
    const enabled = !this.cameraEnabled();
    await room.localParticipant.setCameraEnabled(enabled);
    if (generation !== this.generation || this.room !== room) {
      void room.disconnect();
      return;
    }
    this.cameraEnabled.set(enabled);
  }

  disconnect(clearCall = true): void {
    this.generation += 1;
    const room = this.room;
    this.room = null;
    if (room) void room.disconnect();
    this.clearMedia();
    this.microphoneEnabled.set(true);
    this.cameraEnabled.set(false);
    this.status.set('idle');
    if (clearCall) this.activeCall.set(null);
  }

  private attach(track: RemoteTrack | LocalTrack, local = false): void {
    if (!this.mediaHost) return;
    const element = track.attach();
    if (local) {
      element.muted = true;
      element.dataset['local'] = 'true';
    }
    this.attached.add(element);
    this.mediaHost.append(element);
  }

  private detach(track: RemoteTrack | LocalTrack): void {
    for (const element of track.detach()) {
      this.attached.delete(element);
      element.remove();
    }
  }

  private clearMedia(): void {
    for (const element of this.attached) element.remove();
    this.attached.clear();
    this.mediaHost?.replaceChildren();
    this.mediaHost = null;
  }
}
