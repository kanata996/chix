package runtime

import (
	"errors"
	"net/http"
	"testing"
)

func TestExecutionConfigUsesNearestSingleValueOverrides(t *testing.T) {
	var rootObserverCalls int
	rootObserver := func(Event) {
		rootObserverCalls++
	}
	childObserverCalls := 0
	childObserver := func(Event) {
		childObserverCalls++
	}

	rootExtractor := func(*http.Request) RequestContext {
		return RequestContext{RequestID: "root"}
	}
	grandchildExtractor := func(*http.Request) RequestContext {
		return RequestContext{RequestID: "grandchild"}
	}

	rt := New(
		WithObserver(rootObserver),
		WithExtractor(rootExtractor),
		WithSuccessStatus(http.StatusCreated),
	)
	child := rt.Scope(
		WithObserver(childObserver),
		WithSuccessStatus(http.StatusAccepted),
	)
	grandchild := child.Scope(
		WithExtractor(grandchildExtractor),
	)

	cfg := grandchild.executionConfig()

	gotContext := cfg.extractRequestContext(&http.Request{})
	if gotContext.RequestID != "grandchild" {
		t.Fatalf("expected nearest extractor override, got %+v", gotContext)
	}

	cfg.observer(Event{})
	if childObserverCalls != 1 {
		t.Fatalf("expected child observer to handle event, got %d calls", childObserverCalls)
	}
	if rootObserverCalls != 0 {
		t.Fatalf("expected root observer to be overridden, got %d calls", rootObserverCalls)
	}

	if cfg.successStatus != http.StatusAccepted {
		t.Fatalf("expected nearest success status override, got %d", cfg.successStatus)
	}
}

func TestExecutionConfigAccumulatesMappersInnerToOuter(t *testing.T) {
	baseErr := errors.New("boom")
	var calls []string

	root := New(
		WithErrorMapper(func(err error) *HTTPError {
			if !errors.Is(err, baseErr) {
				t.Fatalf("unexpected root mapper input: %v", err)
			}
			calls = append(calls, "root")
			return nil
		}),
	)
	child := root.Scope(
		WithErrorMapper(func(err error) *HTTPError {
			if !errors.Is(err, baseErr) {
				t.Fatalf("unexpected child mapper input: %v", err)
			}
			calls = append(calls, "child")
			return nil
		}),
	)
	grandchild := child.Scope(
		WithErrorMapper(func(err error) *HTTPError {
			if !errors.Is(err, baseErr) {
				t.Fatalf("unexpected grandchild mapper input: %v", err)
			}
			calls = append(calls, "grandchild")
			return &HTTPError{
				Status:  http.StatusConflict,
				Code:    "grandchild",
				Message: "grandchild",
			}
		}),
	)

	got := grandchild.executionConfig().resolvePublicError(baseErr, nil)

	if got.Code != "grandchild" {
		t.Fatalf("expected innermost mapper to win, got %+v", got)
	}
	if len(calls) != 1 || calls[0] != "grandchild" {
		t.Fatalf("expected mapper evaluation to stop at innermost match, got %+v", calls)
	}
}
