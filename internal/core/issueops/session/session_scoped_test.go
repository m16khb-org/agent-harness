package session

import (
	"fmt"
	"sync"
	"testing"
)

func TestScopedBindingDoesNotClobberPrimary(t *testing.T) {
	dir := t.TempDir()
	store := Store{StateRoot: func() string { return dir }}
	repo := "/repo/scoped"
	parentID := "io-aaaaaaaaaaaa"
	childID := "io-bbbbbbbbbbbb"

	if err := Bind(store, repo, parentID, "123-parent", "/wt/parent"); err != nil {
		t.Fatal(err)
	}
	if err := BindScoped(store, repo, childID, "124-child", "/wt/child"); err != nil {
		t.Fatal(err)
	}

	primary, err := Read(store, repo)
	if err != nil {
		t.Fatal(err)
	}
	if primary.CycleID != parentID || primary.ExpectedWorktree != "/wt/parent" {
		t.Fatalf("scoped bind clobbered primary: got %+v", primary)
	}

	scoped, err := ReadScoped(store, repo, childID)
	if err != nil {
		t.Fatal(err)
	}
	if scoped.CycleID != childID || scoped.ExpectedWorktree != "/wt/child" {
		t.Fatalf("scoped binding mismatch: got %+v", scoped)
	}

	if again, err := readBindingForKey(store, repo, bindingKey(repo)+"-"+childID); err != nil || again.CycleID != childID {
		t.Fatalf("expected scoped binding record to exist: %+v err=%v", again, err)
	}

	bindings, err := ListBindings(store, repo)
	if err != nil {
		t.Fatal(err)
	}
	if len(bindings) != 2 {
		t.Fatalf("expected primary plus one scoped binding, got %#v", bindings)
	}
	if got := bindingsByCycleID(bindings); got[parentID].ExpectedWorktree != "/wt/parent" || got[childID].ExpectedWorktree != "/wt/child" {
		t.Fatalf("unexpected listed bindings: %#v", bindings)
	}
}

func TestUnbindScopedForCycleCompareAndDelete(t *testing.T) {
	dir := t.TempDir()
	store := Store{StateRoot: func() string { return dir }}
	repo := "/repo/scoped-unbind"
	parentID := "io-aaaaaaaaaaaa"
	childA := "io-bbbbbbbbbbbb"
	childB := "io-cccccccccccc"

	if err := Bind(store, repo, parentID, "123-parent", "/wt/parent"); err != nil {
		t.Fatal(err)
	}
	if err := BindScoped(store, repo, childA, "124-child-a", "/wt/child-a"); err != nil {
		t.Fatal(err)
	}
	if err := BindScoped(store, repo, childB, "125-child-b", "/wt/child-b"); err != nil {
		t.Fatal(err)
	}
	if err := UnbindScopedForCycle(store, repo, childA); err != nil {
		t.Fatal(err)
	}

	if got, err := ReadScoped(store, repo, childA); err != nil {
		t.Fatal(err)
	} else if got.CycleID != "" {
		t.Fatalf("expected child A scoped binding removed, got %+v", got)
	}
	if got, err := ReadScoped(store, repo, childB); err != nil {
		t.Fatal(err)
	} else if got.CycleID != childB {
		t.Fatalf("expected child B scoped binding to survive, got %+v", got)
	}
	if got, err := Read(store, repo); err != nil {
		t.Fatal(err)
	} else if got.CycleID != parentID {
		t.Fatalf("expected primary binding to survive scoped unbind, got %+v", got)
	}
	if err := UnbindScopedForCycle(store, repo, childA); err != nil {
		t.Fatal(err)
	}
}

func TestScopedBindingConcurrentBindUnbind(t *testing.T) {
	dir := t.TempDir()
	store := Store{StateRoot: func() string { return dir }}
	repo := "/repo/scoped-race"
	parentID := "io-aaaaaaaaaaaa"
	childA := "io-bbbbbbbbbbbb"
	childB := "io-cccccccccccc"
	childC := "io-dddddddddddd"

	if err := Bind(store, repo, parentID, "123-parent", "/wt/parent"); err != nil {
		t.Fatal(err)
	}

	var wg sync.WaitGroup
	errs := make(chan error, 4)
	for _, child := range []struct {
		id     string
		branch string
		path   string
		unbind bool
	}{
		{id: childA, branch: "124-child-a", path: "/wt/child-a", unbind: true},
		{id: childB, branch: "125-child-b", path: "/wt/child-b"},
		{id: childC, branch: "126-child-c", path: "/wt/child-c"},
	} {
		child := child
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := BindScoped(store, repo, child.id, child.branch, child.path); err != nil {
				errs <- err
				return
			}
			if child.unbind {
				errs <- UnbindScopedForCycle(store, repo, child.id)
			}
		}()
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		errs <- Bind(store, repo, parentID, "123-parent", "/wt/parent")
	}()
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}

	primary, err := Read(store, repo)
	if err != nil {
		t.Fatal(err)
	}
	if primary.CycleID != parentID {
		t.Fatalf("primary binding did not survive concurrent scoped mutations: %+v", primary)
	}
	bindings, err := ListBindings(store, repo)
	if err != nil {
		t.Fatal(err)
	}
	got := bindingsByCycleID(bindings)
	for _, id := range []string{parentID, childB, childC} {
		if got[id].CycleID != id {
			t.Fatalf("surviving binding %s missing from %#v", id, bindings)
		}
	}
	if got[childA].CycleID != "" {
		t.Fatalf("unbound child should not survive: %#v", bindings)
	}
	if len(bindings) != 3 {
		t.Fatalf("expected primary plus two surviving scoped bindings, got %#v", bindings)
	}
}

func TestScopedBindingRejectsUnsafeCycleID(t *testing.T) {
	dir := t.TempDir()
	store := Store{StateRoot: func() string { return dir }}
	for _, id := range []string{"io-short", "io-ABCDEFGHIJKL", "../io-aaaaaaaaaaaa", "io-aaaaaaaaaaaa/extra"} {
		if err := BindScoped(store, "/repo/bad", id, "123-bad", "/wt/bad"); err == nil {
			t.Fatalf("expected scoped bind to reject unsafe cycle id %q", id)
		}
	}
}

func bindingsByCycleID(bindings []Binding) map[string]Binding {
	got := make(map[string]Binding, len(bindings))
	for _, b := range bindings {
		if _, exists := got[b.CycleID]; exists {
			panic(fmt.Sprintf("duplicate binding for cycle %s", b.CycleID))
		}
		got[b.CycleID] = b
	}
	return got
}
