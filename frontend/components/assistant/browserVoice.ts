"use client";

import { useCallback, useEffect, useRef, useState } from "react";

// Dictation and read-aloud from the browser itself: the agents behind the
// chat take no audio. Chrome sends dictated audio to Google to transcribe.

type RecognitionResult = ArrayLike<{ transcript: string }>;

interface Recognition {
  lang: string;
  interimResults: boolean;
  continuous: boolean;
  start(): void;
  stop(): void;
  abort(): void;
  onresult: ((e: { results: ArrayLike<RecognitionResult> }) => void) | null;
  onerror: ((e: { error: string }) => void) | null;
  onend: (() => void) | null;
}

function recognitionCtor(): (new () => Recognition) | null {
  if (typeof window === "undefined") return null;
  const w = window as unknown as { SpeechRecognition?: new () => Recognition; webkitSpeechRecognition?: new () => Recognition };
  return w.SpeechRecognition ?? w.webkitSpeechRecognition ?? null;
}

export function useDictation(onHeard: (text: string) => void, onNotice: (msg: string) => void) {
  const [supported, setSupported] = useState(false);
  const [listening, setListening] = useState(false);
  const recRef = useRef<Recognition | null>(null);
  // The latest callbacks, so the caller can pass plain functions that read
  // fresh state without making start() change on every render.
  const heardRef = useRef(onHeard);
  const noticeRef = useRef(onNotice);
  useEffect(() => {
    heardRef.current = onHeard;
    noticeRef.current = onNotice;
  });

  useEffect(() => {
    // Only known in the browser; the server render always says unsupported.
    // eslint-disable-next-line react-hooks/set-state-in-effect
    setSupported(recognitionCtor() !== null);
    return () => recRef.current?.abort();
  }, []);

  const start = useCallback(() => {
    const Ctor = recognitionCtor();
    if (!Ctor || recRef.current) return;
    const rec = new Ctor();
    rec.lang = "pt-BR";
    rec.interimResults = false;
    rec.continuous = false;
    let heard = "";
    let failed = false;
    rec.onresult = (e) => {
      heard = Array.from(e.results)
        .map((r) => r[0]?.transcript ?? "")
        .join(" ")
        .trim();
    };
    rec.onerror = (e) => {
      failed = true;
      if (e.error === "not-allowed" || e.error === "service-not-allowed") noticeRef.current("O navegador bloqueou o microfone.");
      else if (e.error === "no-speech") noticeRef.current("Não ouvi nada — tente de novo.");
      else if (e.error !== "aborted") noticeRef.current("Não consegui ouvir. Tente de novo.");
    };
    rec.onend = () => {
      recRef.current = null;
      setListening(false);
      if (heard) heardRef.current(heard);
      else if (!failed) noticeRef.current("Não ouvi nada — tente de novo.");
    };
    recRef.current = rec;
    setListening(true);
    rec.start();
  }, []);

  const stop = useCallback(() => recRef.current?.stop(), []);

  return { supported, listening, start, stop };
}

/** Reads text aloud with the first pt-BR voice; false when the browser can't. */
export function speakText(text: string, onEnd: () => void): boolean {
  if (typeof window === "undefined" || !("speechSynthesis" in window)) return false;
  const utterance = new SpeechSynthesisUtterance(text);
  utterance.lang = "pt-BR";
  const voice = window.speechSynthesis.getVoices().find((v) => v.lang.replace("_", "-").startsWith("pt-BR"));
  if (voice) utterance.voice = voice;
  utterance.onend = onEnd;
  utterance.onerror = onEnd;
  window.speechSynthesis.cancel();
  window.speechSynthesis.speak(utterance);
  return true;
}

export function stopSpeech() {
  if (typeof window !== "undefined" && "speechSynthesis" in window) window.speechSynthesis.cancel();
}
