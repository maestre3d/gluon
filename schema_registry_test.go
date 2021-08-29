package gluon

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

var messageRegistryTestCases = []struct {
	In   interface{}
	Want MessageMetadata
}{
	{
		In: struct {
			EntityID    string
			TriggeredAt time.Time
		}{
			EntityID:    "1",
			TriggeredAt: time.Now().UTC(),
		},
		Want: MessageMetadata{
			Topic:         "org.neutrino.dummy.triggered",
			Source:        "",
			SchemaURI:     "",
			SchemaVersion: 1,
		},
	},
	{
		In: sensorDummyEvent{
			SensorID: "abc-1-f",
		},
		Want: MessageMetadata{
			Topic:         "org.neutrino.sensor.activated",
			Source:        "",
			SchemaURI:     "",
			SchemaVersion: 10,
		},
	},
	{
		In: "foo",
		Want: MessageMetadata{
			Topic:         "org.neutrino.string.registered",
			Source:        "",
			SchemaURI:     "",
			SchemaVersion: -1,
		},
	},
}

type sensorDummyEvent struct {
	SensorID string
}

func TestSchemaRegistry_Register(t *testing.T) {
	registry := newSchemaRegistry()
	for _, tt := range messageRegistryTestCases {
		t.Run("", func(t *testing.T) {
			registry.register(tt.In, tt.Want)
			meta, _ := registry.get(tt.In)
			assert.Equal(t, tt.Want.Topic, meta.Topic)
			assert.Equal(t, tt.Want.SchemaURI, meta.SchemaURI)
			assert.Equal(t, tt.Want.SchemaVersion, meta.SchemaVersion)
			assert.Equal(t, tt.Want.Source, meta.Source)
		})
	}
}
