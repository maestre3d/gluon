package glocal

import (
	"github.com/maestre3d/gluon"
)

type partition struct {
	totalMessages int
	lastMessage   gluon.TransportMessage
}

func newPartition() *partition {
	return &partition{
		totalMessages: 0,
	}
}

func (p *partition) push(msg *gluon.TransportMessage) {
	p.lastMessage = *msg
	p.totalMessages++
}
