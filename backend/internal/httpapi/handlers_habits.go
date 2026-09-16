package httpapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/rafael/estus-vault/backend/internal/domain"
	"github.com/rafael/estus-vault/backend/internal/service"
)

// Days cross the API as "YYYY-MM-DD" and weekdays as a list (0 = Sunday), as
// in Treino. "Today" is the caller's: the backend has no time zone of its own.

type habitDTO struct {
	ID            string         `json:"id"`
	Name          string         `json:"name"`
	Kind          string         `json:"kind"`
	Target        int            `json:"target"`
	Unit          string         `json:"unit"`
	Days          []int          `json:"days"`
	Color         string         `json:"color"`
	Archived      bool           `json:"archived"`
	StartDay      string         `json:"start_day"`
	CurrentStreak int            `json:"current_streak"`
	BestStreak    int            `json:"best_streak"`
	Logs          map[string]int `json:"logs"`
	CreatedAt     string         `json:"created_at"`
}

// toHabitDTO sends only the log from `from` on — the screens show a year at
// most — while the streaks are worked out from all of it.
func toHabitDTO(h domain.Habit, today, from time.Time) habitDTO {
	current, best := h.Streaks(today)
	logs := map[string]int{}
	for day, count := range h.Logs {
		if d, err := time.Parse(domain.DayLayout, day); err == nil && !d.Before(from) && !d.After(today) {
			logs[day] = count
		}
	}
	return habitDTO{
		ID:            h.ID,
		Name:          h.Name,
		Kind:          string(h.Kind),
		Target:        h.Target,
		Unit:          h.Unit,
		Days:          domain.MaskToDays(h.Weekdays),
		Color:         h.Color,
		Archived:      h.Archived,
		StartDay:      h.StartDay.Format(domain.DayLayout),
		CurrentStreak: current,
		BestStreak:    best,
		Logs:          logs,
		CreatedAt:     h.CreatedAt.Format(time.RFC3339),
	}
}

type habitRequest struct {
	Name     string `json:"name"`
	Kind     string `json:"kind"`
	Target   int    `json:"target"`
	Unit     string `json:"unit"`
	Days     []int  `json:"days"`
	Color    string `json:"color"`
	Archived bool   `json:"archived"`
	StartDay string `json:"start_day"`
}

func decodeHabit(r *http.Request, needStart bool) (domain.Habit, error) {
	var req habitRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return domain.Habit{}, fmt.Errorf("%w: invalid JSON body", domain.ErrValidation)
	}
	mask, err := domain.DaysToMask(req.Days)
	if err != nil {
		return domain.Habit{}, err
	}
	h := domain.Habit{
		Name:     req.Name,
		Kind:     domain.HabitKind(req.Kind),
		Target:   req.Target,
		Unit:     req.Unit,
		Weekdays: mask,
		Color:    req.Color,
		Archived: req.Archived,
	}
	if needStart {
		if h.StartDay, err = domain.ParseDay(req.StartDay); err != nil {
			return domain.Habit{}, err
		}
	}
	return h, nil
}

type HabitHandlers struct {
	habits *service.HabitService
}

func NewHabitHandlers(habits *service.HabitService) *HabitHandlers {
	return &HabitHandlers{habits: habits}
}

// List takes ?today=YYYY-MM-DD (required) and ?days=N, how many days of log
// to send back ending today (default 7, at most 400).
func (h *HabitHandlers) List(w http.ResponseWriter, r *http.Request) {
	today, err := domain.ParseDay(r.URL.Query().Get("today"))
	if err != nil {
		writeError(w, err)
		return
	}
	days := 7
	if raw := r.URL.Query().Get("days"); raw != "" {
		if _, err := fmt.Sscanf(raw, "%d", &days); err != nil || days < 1 || days > 400 {
			writeError(w, fmt.Errorf("%w: days must be between 1 and 400", domain.ErrValidation))
			return
		}
	}
	from := today.AddDate(0, 0, -(days - 1))

	habits, err := h.habits.List(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	out := make([]habitDTO, len(habits))
	for i, hb := range habits {
		out[i] = toHabitDTO(hb, today, from)
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *HabitHandlers) Create(w http.ResponseWriter, r *http.Request) {
	in, err := decodeHabit(r, true)
	if err != nil {
		writeError(w, err)
		return
	}
	saved, err := h.habits.Create(r.Context(), in)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, toHabitDTO(saved, saved.StartDay, saved.StartDay))
}

func (h *HabitHandlers) Update(w http.ResponseWriter, r *http.Request) {
	in, err := decodeHabit(r, false)
	if err != nil {
		writeError(w, err)
		return
	}
	if err := h.habits.Update(r.Context(), r.PathValue("id"), in); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusNoContent, nil)
}

func (h *HabitHandlers) Delete(w http.ResponseWriter, r *http.Request) {
	if err := h.habits.Delete(r.Context(), r.PathValue("id")); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusNoContent, nil)
}

type habitLogRequest struct {
	Day   string `json:"day"`
	Count int    `json:"count"`
}

func (h *HabitHandlers) SetLog(w http.ResponseWriter, r *http.Request) {
	var req habitLogRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, fmt.Errorf("%w: invalid JSON body", domain.ErrValidation))
		return
	}
	day, err := domain.ParseDay(req.Day)
	if err != nil {
		writeError(w, err)
		return
	}
	if err := h.habits.SetLog(r.Context(), r.PathValue("id"), day, req.Count); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusNoContent, nil)
}
