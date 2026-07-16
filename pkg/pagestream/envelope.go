package pagestream

// Envelope carries delivery and trace metadata beside a Datastar signal
// patch. Metadata never enters the browser's signal graph.
type Envelope struct {
	Signals  SignalPatch
	Delivery DeliveryMetadata
	Trace    TraceMetadata

	trace *traceSpan
}

// DeliveryMetadata defines ordering and coalescing without inspecting signal
// payload shapes. Generation zero means the message is not generation scoped.
type DeliveryMetadata struct {
	Generation    uint64
	Boundary      bool
	CoalesceGroup string
	MergeRoots    []string
}

// TraceMetadata identifies the producer and its domain correlation ID.
type TraceMetadata struct {
	Origin        string
	CorrelationID string
}

type traceSpan struct {
	store       *TraceStore
	streamID    string
	sequence    uint64
	publishedAt int64
	coalesced   int
}
