package assistant

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/rafael/estus-vault/backend/internal/domain"
	"github.com/rafael/estus-vault/backend/internal/service"
)

func TestFolderPathsAndFind(t *testing.T) {
	casa, contas := "c", "k"
	folders := []domain.DocumentFolder{
		{ID: casa, Name: "Casa"},
		{ID: contas, Name: "Contas", ParentID: &casa},
		{ID: "t", Name: "Contas", ParentID: nil},
	}
	paths := folderPaths(folders)
	if paths[contas] != "Casa/Contas" || paths["t"] != "Contas" {
		t.Fatalf("paths = %v", paths)
	}
	for query, want := range map[string]string{"casa/contas": contas, "/Casa/Contas/": contas, "Contas": "t", "k": contas, "cas": casa} {
		got, err := findFolder(folders, query)
		if err != nil || got == nil || *got != want {
			t.Errorf("findFolder(%q) = %v, %v; want %s", query, got, err, want)
		}
	}
	for _, root := range []string{"", "  ", "Início", "inicio"} {
		if got, err := findFolder(folders, root); err != nil || got != nil {
			t.Errorf("findFolder(%q) = %v, %v; want root", root, got, err)
		}
	}
	if _, err := findFolder(folders, "nada"); !errors.Is(err, domain.ErrValidation) {
		t.Errorf("unknown folder err = %v", err)
	}
}

func TestFolderPathsSurvivesCycle(t *testing.T) {
	a, b := "a", "b"
	paths := folderPaths([]domain.DocumentFolder{{ID: a, Name: "A", ParentID: &b}, {ID: b, Name: "B", ParentID: &a}})
	if paths[a] != "B/A" || paths[b] != "A/B" {
		t.Fatalf("paths = %v", paths)
	}
}

func TestWorkoutPatchKeepsOmittedFields(t *testing.T) {
	current := domain.Workout{ID: "w", Name: "Treino A", Focus: "peito", Weekdays: 0b0000010, Exercises: []domain.Exercise{{Name: "Supino", Sets: 4, Reps: "8"}}}

	var in workoutIn
	if err := json.Unmarshal([]byte(`{"id":"w","focus":" costas ","days":[1,3]}`), &in); err != nil {
		t.Fatal(err)
	}
	got, err := in.apply(current)
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "Treino A" || got.Focus != "costas" || got.Weekdays != 0b0001010 || len(got.Exercises) != 1 {
		t.Fatalf("patched = %+v", got)
	}

	if err := json.Unmarshal([]byte(`{"exercises":[]}`), &in); err != nil {
		t.Fatal(err)
	}
	if got, _ = in.apply(current); len(got.Exercises) != 0 {
		t.Fatalf("empty exercises list should clear, got %+v", got.Exercises)
	}

	in = workoutIn{Days: &[]int{7}}
	if _, err := in.apply(current); !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("day 7 err = %v", err)
	}
}

func TestMealPatchReplacesFoods(t *testing.T) {
	current := domain.Meal{Name: "Almoço", Time: "12:30", Weekdays: 127, Items: []domain.MealItem{{Food: "Arroz", Kcal: 200}}}
	var in mealIn
	if err := json.Unmarshal([]byte(`{"time":"13:00","foods":[{"food":"Frango","quantity":"150 g","kcal":250,"protein_g":45}]}`), &in); err != nil {
		t.Fatal(err)
	}
	got, err := in.apply(current)
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "Almoço" || got.Time != "13:00" || got.Weekdays != 127 || len(got.Items) != 1 || got.Items[0].Food != "Frango" || got.Items[0].ProteinG != 45 {
		t.Fatalf("patched = %+v", got)
	}
}

func TestNextNotebookColor(t *testing.T) {
	if got := nextNotebookColor(nil); got != notebookColors[0] {
		t.Fatalf("first = %s", got)
	}
	if got := nextNotebookColor([]string{"#8B5CF6"}); got != notebookColors[1] {
		t.Fatalf("case-insensitive skip = %s", got)
	}
	if got := nextNotebookColor(append([]string{}, notebookColors...)); got != notebookColors[0] {
		t.Fatalf("wraps = %s", got)
	}
}

// The write tools only register when their service exists; zero-value
// services are enough to check the schemas without a database.
func TestCrudToolSchemas(t *testing.T) {
	r := New(Deps{
		Training: &service.TrainingService{}, Diet: &service.DietService{},
		Documents: &service.DocumentService{}, Boards: &service.BoardService{},
		Notes: &service.NoteService{}, NoteCategories: &service.NoteCategoryService{},
	})
	destructive := map[string]bool{
		"training_delete": true, "diet_delete": true, "documents_folder_delete": true, "documents_delete": true,
		"boards_delete": true, "notes_notebook_delete": true,
	}
	for _, name := range []string{
		"training_create", "training_update", "training_delete",
		"diet_create", "diet_update", "diet_delete", "diet_targets", "diet_set_targets",
		"documents_folders", "documents_folder_create", "documents_folder_rename", "documents_folder_delete", "documents_move", "documents_delete",
		"boards_create", "boards_rename", "boards_delete",
		"notes_update", "notes_notebooks", "notes_notebook_create", "notes_notebook_rename", "notes_notebook_delete",
	} {
		tool, ok := r.Get(name)
		if !ok {
			t.Errorf("%s not registered", name)
			continue
		}
		if tool.Destructive != destructive[name] {
			t.Errorf("%s destructive = %v", name, tool.Destructive)
		}
		props := tool.Input["properties"].(map[string]any)
		for _, req := range tool.Input["required"].([]string) {
			if _, ok := props[req]; !ok {
				t.Errorf("%s requires %q but has no such property", name, req)
			}
		}
		if _, err := json.Marshal(tool.Input); err != nil {
			t.Errorf("%s schema: %v", name, err)
		}
	}
}
