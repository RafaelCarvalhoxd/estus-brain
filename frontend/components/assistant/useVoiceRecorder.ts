"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { recordingToWav } from "./wav";

// Click to talk: records until stop() is called, the person goes quiet for a
// moment after speaking, or the time cap is reached; then hands over a WAV.

const MAX_MS = 120_000;
const SILENCE_MS = 2000;
const SPEECH_LEVEL = 0.02; // RMS above this counts as someone speaking
// Anything this long counts as speech even if no frame ever heard it: a hidden
// tab runs no animation frames, and a quiet microphone never reaches the level.
const MIN_SPEECH_MS = 1500;

export type RecorderState = "idle" | "recording" | "processing";

export function useVoiceRecorder(onRecorded: (wav: Blob) => Promise<void>, onError: (message: string) => void) {
  const [state, setState] = useState<RecorderState>("idle");
  const [elapsed, setElapsed] = useState(0);
  const stopRef = useRef<(() => void) | null>(null);
  const handlers = useRef({ onRecorded, onError });
  useEffect(() => {
    handlers.current = { onRecorded, onError };
  });

  const start = useCallback(async () => {
    if (stopRef.current) return;
    let stream: MediaStream;
    try {
      stream = await navigator.mediaDevices.getUserMedia({ audio: { echoCancellation: true, noiseSuppression: true } });
    } catch {
      handlers.current.onError("Permita o microfone para este site nas configurações do navegador.");
      return;
    }
    let recorder: MediaRecorder;
    let audio: AudioContext | undefined;
    let analyser: AnalyserNode;
    try {
      recorder = new MediaRecorder(stream);
      audio = new AudioContext();
      analyser = audio.createAnalyser();
      analyser.fftSize = 1024;
      audio.createMediaStreamSource(stream).connect(analyser);
    } catch {
      stream.getTracks().forEach((track) => track.stop());
      void audio?.close();
      handlers.current.onError("Não consegui gravar neste navegador.");
      return;
    }
    const chunks: Blob[] = [];
    const levels = new Float32Array(analyser.fftSize);
    const startedAt = performance.now();
    let spoke = false;
    let quietSince = startedAt;
    let frame = 0;
    let failed = false;
    let released = false;

    // Frees the microphone and the analyser; safe to call more than once.
    const release = () => {
      if (released) return;
      released = true;
      cancelAnimationFrame(frame);
      clearTimeout(cap);
      stream.getTracks().forEach((track) => track.stop());
      void audio.close();
    };

    const stop = () => {
      if (stopRef.current !== stop) return;
      stopRef.current = null;
      cancelAnimationFrame(frame);
      clearTimeout(cap);
      if (recorder.state !== "inactive") {
        recorder.stop(); // onstop does the rest
        return;
      }
      // Nothing will fire onstop, so don't leave the button stuck on "recording".
      release();
      setState("idle");
    };
    stopRef.current = stop;
    // A hidden tab runs no animation frames, so the cap needs its own timer.
    const cap = setTimeout(stop, MAX_MS);

    const watch = () => {
      const now = performance.now();
      analyser.getFloatTimeDomainData(levels);
      let sum = 0;
      for (const v of levels) sum += v * v;
      if (Math.sqrt(sum / levels.length) > SPEECH_LEVEL) {
        spoke = true;
        quietSince = now;
      }
      setElapsed(Math.floor((now - startedAt) / 1000));
      if (now - startedAt >= MAX_MS || (spoke && now - quietSince >= SILENCE_MS)) {
        stop();
        return;
      }
      frame = requestAnimationFrame(watch);
    };

    recorder.ondataavailable = (e) => {
      if (e.data.size > 0) chunks.push(e.data);
    };
    recorder.onerror = () => {
      if (stopRef.current === stop) stopRef.current = null;
      failed = true;
      release();
      setState("idle");
      handlers.current.onError("Não consegui gravar neste navegador.");
    };
    recorder.onstop = async () => {
      release();
      if (failed) return; // onerror already said what happened
      if (!spoke && performance.now() - startedAt < MIN_SPEECH_MS) {
        setState("idle");
        handlers.current.onError("Não ouvi nada — tente de novo.");
        return;
      }
      setState("processing");
      try {
        const { wav } = await recordingToWav(new Blob(chunks, { type: recorder.mimeType }));
        await handlers.current.onRecorded(wav);
      } catch {
        handlers.current.onError("Não consegui ler a gravação. Tente de novo.");
      } finally {
        setState("idle");
      }
    };

    recorder.start();
    setElapsed(0);
    setState("recording");
    frame = requestAnimationFrame(watch);
  }, []);

  const stop = useCallback(() => stopRef.current?.(), []);

  // Leaving the chat mid-recording releases the microphone.
  useEffect(() => () => stopRef.current?.(), []);

  return { state, elapsed, start, stop };
}
