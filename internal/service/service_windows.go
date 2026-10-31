//go:build windows

package service

import (
	"context"
	"fmt"
	"time"

	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/mgr"
)

func install(spec Spec) error {
	m, err := mgr.Connect()
	if err != nil {
		return fmt.Errorf("connect to service manager (run as Administrator): %w", err)
	}
	defer m.Disconnect()
	if s, err := m.OpenService(spec.Name); err == nil {
		s.Close()
		return fmt.Errorf("service %q already exists; run uninstall-service first", spec.Name)
	}
	s, err := m.CreateService(spec.Name, spec.Exe, mgr.Config{
		DisplayName:      spec.DisplayName,
		Description:      spec.Description,
		StartType:        mgr.StartAutomatic,
		DelayedAutoStart: true,
		// A virtual account: least privilege, no password to manage (docs/reference/12 §2.4).
		ServiceStartName: `NT SERVICE\` + spec.Name,
	}, spec.Args...)
	if err != nil {
		return fmt.Errorf("create service: %w", err)
	}
	defer s.Close()
	// Restart after a crash: 5 s, then 30 s, then 60 s.
	return s.SetRecoveryActions([]mgr.RecoveryAction{
		{Type: mgr.ServiceRestart, Delay: 5 * time.Second},
		{Type: mgr.ServiceRestart, Delay: 30 * time.Second},
		{Type: mgr.ServiceRestart, Delay: 60 * time.Second},
	}, 24*60*60)
}

func uninstall(name string) error {
	m, err := mgr.Connect()
	if err != nil {
		return fmt.Errorf("connect to service manager (run as Administrator): %w", err)
	}
	defer m.Disconnect()
	s, err := m.OpenService(name)
	if err != nil {
		return fmt.Errorf("open service %q: %w", name, err)
	}
	defer s.Close()
	if st, err := s.Query(); err == nil && st.State != svc.Stopped {
		_, _ = s.Control(svc.Stop)
		for i := 0; i < 60; i++ { // wait up to 30 s for a graceful stop
			if st, err := s.Query(); err != nil || st.State == svc.Stopped {
				break
			}
			time.Sleep(500 * time.Millisecond)
		}
	}
	return s.Delete()
}

func runAsService(name string, main func(context.Context) error) (bool, error) {
	isService, err := svc.IsWindowsService()
	if err != nil || !isService {
		return false, err
	}
	h := &handler{main: main}
	if err := svc.Run(name, h); err != nil {
		return true, err
	}
	return true, h.err
}

type handler struct {
	main func(context.Context) error
	err  error
}

// Execute implements svc.Handler: it runs main and cancels it on Stop or Shutdown.
func (h *handler) Execute(_ []string, req <-chan svc.ChangeRequest, status chan<- svc.Status) (bool, uint32) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	status <- svc.Status{State: svc.StartPending}
	go func() { done <- h.main(ctx) }()
	status <- svc.Status{State: svc.Running, Accepts: svc.AcceptStop | svc.AcceptShutdown}
	for {
		select {
		case h.err = <-done:
			status <- svc.Status{State: svc.StopPending}
			if h.err != nil {
				return true, 1 // non-zero exit makes the SCM apply the recovery actions
			}
			return false, 0
		case c := <-req:
			switch c.Cmd {
			case svc.Interrogate:
				status <- c.CurrentStatus
			case svc.Stop, svc.Shutdown:
				status <- svc.Status{State: svc.StopPending, WaitHint: 45000}
				cancel()
			}
		}
	}
}
