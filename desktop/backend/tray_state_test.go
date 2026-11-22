package backend

import (
	"context"
	"errors"
	"testing"
)

func TestTrayPreparationFailureAndCancellation(t *testing.T) {
	e := &Engine{}
	if err := e.Connect(Server{}); err == nil {
		t.Fatal("expected local validation error")
	}
	_, _, _, _, preparing, failed := e.TraySnapshot()
	if preparing || !failed {
		t.Fatal("pre-core connection error is invisible to tray")
	}
	e.Disconnect()
	_, _, _, _, preparing, failed = e.TraySnapshot()
	if preparing || failed {
		t.Fatal("user cancellation retained a tray error")
	}
}

func TestOldPlanCannotOverwriteNewTrayPresentation(t *testing.T) {
	e := &Engine{planGeneration: 2, preparing: true}
	e.finishConnectPlan(1, errors.New("old request"))
	if !e.preparing || e.preparationFailed {
		t.Fatal("old failure replaced a newer preparation")
	}
	e.finishConnectPlan(2, context.Canceled)
	if e.preparing || e.preparationFailed {
		t.Fatal("cancellation displayed as an error")
	}
}
