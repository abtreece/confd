package main

import (
	"errors"
	"fmt"
	"testing"

	"github.com/abtreece/confd/pkg/template"
)

// A processor sends its last error and then closes doneChan, so the error is
// still unread when waitDone sees doneChan closed. It must not be lost.
func TestProcessorErrors_WaitDoneKeepsLoadFailureSentBeforeDone(t *testing.T) {
	loadFailure := fmt.Errorf("%w: bad.toml", template.ErrNoTemplateResources)
	// Repeated because select picks at random between two ready channels.
	for i := 0; i < 100; i++ {
		errChan := make(chan error, 10)
		doneChan := make(chan bool)
		errChan <- loadFailure
		close(doneChan)

		procErrs := &processorErrors{errChan: errChan, doneChan: doneChan}
		procErrs.waitDone()

		if !errors.Is(procErrs.fatal, template.ErrNoTemplateResources) {
			t.Fatalf("iteration %d: fatal = %v, want the load failure", i, procErrs.fatal)
		}
	}
}

func TestProcessorErrors_OtherErrorsAreNotFatal(t *testing.T) {
	errChan := make(chan error, 10)
	doneChan := make(chan bool)
	errChan <- errors.New("watch error")
	close(doneChan)

	procErrs := &processorErrors{errChan: errChan, doneChan: doneChan}
	procErrs.waitDone()

	if procErrs.fatal != nil {
		t.Errorf("fatal = %v, want nil", procErrs.fatal)
	}
	if len(errChan) != 0 {
		t.Errorf("%d errors left unread", len(errChan))
	}
}

// waitDone keeps reading while it waits, so a processor blocked on a full
// error channel can still stop.
func TestProcessorErrors_WaitDoneUnblocksSender(t *testing.T) {
	errChan := make(chan error)
	doneChan := make(chan bool)
	go func() {
		errChan <- template.ErrNoTemplateResources
		close(doneChan)
	}()

	procErrs := &processorErrors{errChan: errChan, doneChan: doneChan}
	procErrs.waitDone()

	if !errors.Is(procErrs.fatal, template.ErrNoTemplateResources) {
		t.Errorf("fatal = %v, want the load failure", procErrs.fatal)
	}
}
