package httpapi

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/rafael/estus-vault/backend/internal/domain"
	"github.com/rafael/estus-vault/backend/internal/service"
)

type TrainingHandlers struct {
	training *service.TrainingService
}

func NewTrainingHandlers(training *service.TrainingService) *TrainingHandlers {
	return &TrainingHandlers{training: training}
}

func (h *TrainingHandlers) List(w http.ResponseWriter, r *http.Request) {
	workouts, err := h.training.List(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	out := make([]workoutDTO, len(workouts))
	for i, wk := range workouts {
		out[i] = toWorkoutDTO(wk)
	}
	writeJSON(w, http.StatusOK, out)
}

func decodeWorkout(r *http.Request) (domain.Workout, error) {
	var req workoutRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return domain.Workout{}, fmt.Errorf("%w: invalid JSON body", domain.ErrValidation)
	}
	return req.toDomain()
}

func (h *TrainingHandlers) Create(w http.ResponseWriter, r *http.Request) {
	wk, err := decodeWorkout(r)
	if err != nil {
		writeError(w, err)
		return
	}
	saved, err := h.training.Create(r.Context(), wk)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, toWorkoutDTO(saved))
}

func (h *TrainingHandlers) Update(w http.ResponseWriter, r *http.Request) {
	wk, err := decodeWorkout(r)
	if err != nil {
		writeError(w, err)
		return
	}
	saved, err := h.training.Update(r.Context(), r.PathValue("id"), wk)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toWorkoutDTO(saved))
}

func (h *TrainingHandlers) Delete(w http.ResponseWriter, r *http.Request) {
	if err := h.training.Delete(r.Context(), r.PathValue("id")); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusNoContent, nil)
}

type DietHandlers struct {
	diet *service.DietService
}

func NewDietHandlers(diet *service.DietService) *DietHandlers {
	return &DietHandlers{diet: diet}
}

func (h *DietHandlers) List(w http.ResponseWriter, r *http.Request) {
	meals, err := h.diet.List(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	out := make([]mealDTO, len(meals))
	for i, m := range meals {
		out[i] = toMealDTO(m)
	}
	writeJSON(w, http.StatusOK, out)
}

func decodeMeal(r *http.Request) (domain.Meal, error) {
	var req mealRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return domain.Meal{}, fmt.Errorf("%w: invalid JSON body", domain.ErrValidation)
	}
	return req.toDomain()
}

func (h *DietHandlers) Create(w http.ResponseWriter, r *http.Request) {
	m, err := decodeMeal(r)
	if err != nil {
		writeError(w, err)
		return
	}
	saved, err := h.diet.Create(r.Context(), m)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, toMealDTO(saved))
}

func (h *DietHandlers) Update(w http.ResponseWriter, r *http.Request) {
	m, err := decodeMeal(r)
	if err != nil {
		writeError(w, err)
		return
	}
	saved, err := h.diet.Update(r.Context(), r.PathValue("id"), m)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toMealDTO(saved))
}

func (h *DietHandlers) Delete(w http.ResponseWriter, r *http.Request) {
	if err := h.diet.Delete(r.Context(), r.PathValue("id")); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusNoContent, nil)
}

func (h *DietHandlers) Targets(w http.ResponseWriter, r *http.Request) {
	t, err := h.diet.Targets(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, dietTargetsDTO{Kcal: t.Kcal, ProteinG: t.ProteinG, CarbsG: t.CarbsG, FatG: t.FatG})
}

func (h *DietHandlers) SetTargets(w http.ResponseWriter, r *http.Request) {
	var req dietTargetsDTO
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, fmt.Errorf("%w: invalid JSON body", domain.ErrValidation))
		return
	}
	t, err := h.diet.SetTargets(r.Context(), domain.DietTargets{Kcal: req.Kcal, ProteinG: req.ProteinG, CarbsG: req.CarbsG, FatG: req.FatG})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, dietTargetsDTO{Kcal: t.Kcal, ProteinG: t.ProteinG, CarbsG: t.CarbsG, FatG: t.FatG})
}
