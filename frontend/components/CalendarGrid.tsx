"use client";

import { useState } from "react";
import type { CalendarDay } from "@/lib/month";
import type { Event } from "@/lib/agenda";
import { Modal } from "./Modal";
import { NewEventForm, EventRow } from "./EventForm";
import { IconPlus } from "./icons";

const WEEKDAY_LABELS = ["Dom", "Seg", "Ter", "Qua", "Qui", "Sex", "Sáb"];
const MAX_VISIBLE_PER_DAY = 3;

function eventTime(iso: string): string {
  return new Date(iso).toLocaleTimeString("pt-BR", { hour: "2-digit", minute: "2-digit" });
}

function dayLabel(iso: string): string {
  const label = new Date(`${iso}T00:00:00`).toLocaleDateString("pt-BR", {
    weekday: "long",
    day: "2-digit",
    month: "long",
  });
  return label.charAt(0).toUpperCase() + label.slice(1);
}

export function CalendarGrid({ days, eventsByDay }: { days: CalendarDay[]; eventsByDay: Record<string, Event[]> }) {
  const [createDate, setCreateDate] = useState<string | null>(null);
  const [dayOverflow, setDayOverflow] = useState<string | null>(null);
  const [editingEvent, setEditingEvent] = useState<Event | null>(null);

  return (
    <div className="cal-grid-wrap">
      <div className="cal-weekdays">
        {WEEKDAY_LABELS.map((label) => (
          <div key={label} className="cal-weekday">
            {label}
          </div>
        ))}
      </div>

      <div className="cal-grid">
        {days.map((day) => {
          const dayEvents = eventsByDay[day.iso] ?? [];
          const visible = dayEvents.slice(0, MAX_VISIBLE_PER_DAY);
          const overflowCount = dayEvents.length - visible.length;

          return (
            <div key={day.iso} className={`cal-cell ${day.inMonth ? "" : "cal-cell-outside"}`}>
              <div className="cal-cell-head">
                <span className={`cal-daynum ${day.isToday ? "cal-daynum-today" : ""}`}>{day.dayNumber}</span>
                <button
                  type="button"
                  className="cal-add-btn"
                  aria-label="Novo evento neste dia"
                  onClick={() => setCreateDate(day.iso)}
                >
                  <IconPlus />
                </button>
              </div>
              <div className="cal-cell-events">
                {visible.map((e) => (
                  <button key={e.id} type="button" className="cal-event-pill" onClick={() => setEditingEvent(e)}>
                    <span className="cal-event-time">{eventTime(e.starts_at)}</span>
                    <span className="cal-event-title">{e.title}</span>
                  </button>
                ))}
                {overflowCount > 0 && (
                  <button type="button" className="cal-event-more" onClick={() => setDayOverflow(day.iso)}>
                    +{overflowCount} mais
                  </button>
                )}
              </div>
            </div>
          );
        })}
      </div>

      <Modal open={createDate !== null} onClose={() => setCreateDate(null)}>
        {createDate && <NewEventForm defaultDate={createDate} onSuccess={() => setCreateDate(null)} />}
      </Modal>

      <Modal open={editingEvent !== null} onClose={() => setEditingEvent(null)}>
        {editingEvent && (
          <div className="panel">
            <div className="panel-head">
              <h2>{dayLabel(editingEvent.starts_at.slice(0, 10))}</h2>
            </div>
            <div className="event-list">
              <EventRow event={editingEvent} />
            </div>
          </div>
        )}
      </Modal>

      <Modal open={dayOverflow !== null} onClose={() => setDayOverflow(null)}>
        {dayOverflow && (
          <div className="panel">
            <div className="panel-head">
              <h2>{dayLabel(dayOverflow)}</h2>
            </div>
            <div className="event-list">
              {(eventsByDay[dayOverflow] ?? []).map((e) => (
                <EventRow key={e.id} event={e} />
              ))}
            </div>
          </div>
        )}
      </Modal>
    </div>
  );
}
