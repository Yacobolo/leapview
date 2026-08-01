# API compatibility baselines

`control-v1.binpb` is the committed Buf image for the first stable
`rtest.v1.ControlService` contract. `task proto:check` compares current protobuf sources
against it and rejects wire-incompatible changes. Compatible additions do not replace the
baseline; incompatible evolution belongs in a new versioned protobuf package.
