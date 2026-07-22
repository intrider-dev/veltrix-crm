import { DestroyRef, inject, Injectable, signal } from '@angular/core';
import type { LocalTrack, RemoteTrack, Room } from 'livekit-client';

import type { Call, CallJoin } from '../../core/api/api.types';

@Injectable({ providedIn: 'root' })
export class CallSessionService {
  private room: Room | null = null;
  private mediaHost: HTMLElement | null = null;
  private readonly attached = new Set<HTMLMediaElement>();

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
    this.activeCall.set(grant.call);
    this.status.set('connecting');
    this.error.set(null);
    this.mediaHost = mediaHost;
    try {
      const { Room, RoomEvent, Track } = await import('livekit-client');
      const room = new Room({ adaptiveStream: true, dynacast: true });
      this.room = room;
      room.on(RoomEvent.TrackSubscribed, (track) => this.attach(track));
      room.on(RoomEvent.TrackUnsubscribed, (track) => this.detach(track));
      room.on(RoomEvent.Disconnected, () => {
        this.clearMedia();
        this.status.set('idle');
        this.activeCall.set(null);
      });
      await room.connect(grant.url, grant.token);
      await room.localParticipant.setMicrophoneEnabled(true);
      this.microphoneEnabled.set(true);
      if (grant.call.kind === 'video') {
        await room.localParticipant.setCameraEnabled(true);
        this.cameraEnabled.set(true);
        const localCamera = room.localParticipant.getTrackPublication(Track.Source.Camera)?.track;
        if (localCamera) this.attach(localCamera, true);
      }
      this.status.set('connected');
    } catch (error) {
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
    if (!this.room) return;
    const enabled = !this.microphoneEnabled();
    await this.room.localParticipant.setMicrophoneEnabled(enabled);
    this.microphoneEnabled.set(enabled);
  }

  async toggleCamera(): Promise<void> {
    if (!this.room || this.activeCall()?.kind !== 'video') return;
    const enabled = !this.cameraEnabled();
    await this.room.localParticipant.setCameraEnabled(enabled);
    this.cameraEnabled.set(enabled);
  }

  disconnect(clearCall = true): void {
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
