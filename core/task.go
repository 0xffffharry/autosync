package core

import "context"

type Task struct {
	ctx     context.Context
	cancel  context.CancelFunc
	path    string
	ch      chan struct{}
	closeCh chan struct{}
	doFunc  func(context.Context, string)
}

func NewTask(ctx context.Context, path string, doFunc func(context.Context, string)) *Task {
	ctx, cancel := context.WithCancel(ctx)
	t := &Task{
		ctx:     ctx,
		cancel:  cancel,
		path:    path,
		ch:      make(chan struct{}, 1),
		closeCh: make(chan struct{}, 1),
		doFunc:  doFunc,
	}
	go t.loopHandle()
	return t
}

func (t *Task) loopHandle() {
	defer func() {
		t.closeCh <- struct{}{}
	}()
	for {
		select {
		case <-t.ctx.Done():
			return
		case <-t.ch:
			t.doFunc(t.ctx, t.path)
		}
	}
}

func (t *Task) Close() {
	t.cancel()
	<-t.closeCh
	close(t.closeCh)
	close(t.ch)
}

func (t *Task) Call() {
	select {
	case <-t.ctx.Done():
	default:
		select {
		case t.ch <- struct{}{}:
		default:
		}
	}
}
