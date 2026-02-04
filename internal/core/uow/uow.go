package uow

import (
	"errors"
	"fmt"
)

// multiErr накапливает ошибки при cleanup операциях
type multiErr struct {
	primary error
	cleanup []error
}

func (m *multiErr) addCleanup(err error) {
	if err != nil {
		m.cleanup = append(m.cleanup, err)
	}
}

func (m *multiErr) error() error {
	if m.primary == nil {
		return nil
	}
	if len(m.cleanup) == 0 {
		return m.primary
	}
	return errors.Join(append([]error{m.primary}, m.cleanup...)...)
}

// Uow управляет откатом ресурсов при ошибках
type Uow struct {
	cleanups []func() error
}

func UnitOfWork() *Uow {
	return &Uow{}
}

func (r *Uow) Add(name string, fn func() error) {
	r.cleanups = append(r.cleanups, func() error {
		if err := fn(); err != nil {
			return fmt.Errorf("cleanup %s: %w", name, err)
		}
		return nil
	})
}

func (r *Uow) Rollback(primary error) error {
	me := &multiErr{primary: primary}
	// откатываем в обратном порядке (LIFO)
	for i := len(r.cleanups) - 1; i >= 0; i-- {
		me.addCleanup(r.cleanups[i]())
	}
	return me.error()
}

func (r *Uow) Commit() {
	r.cleanups = nil // ресурсы успешно созданы, cleanup не нужен
}
