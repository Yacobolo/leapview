package module

import "github.com/Yacobolo/leapview/pkg/pagestream"

// PublishSemanticModelRefresh asks every active dashboard bound to the model to
// refresh and publishes the durable refresh timestamp to its page stream.
func (m *Module) PublishSemanticModelRefresh(workspaceID, environment, modelID, refreshedAt string) {
	if m == nil || m.coordinators == nil {
		return
	}
	for _, streamID := range m.coordinators.RefreshSemanticModel(workspaceID, environment, modelID) {
		if m.handler.Broker != nil {
			m.handler.Broker.PublishEnvelope(streamID, pagestream.Envelope{
				Signals: pagestream.SignalPatch{"status": map[string]any{"lastUpdated": refreshedAt}},
			})
		}
	}
}
