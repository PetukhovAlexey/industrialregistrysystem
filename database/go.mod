module industrialregistrysystem/database

go 1.25.3

replace industrialregistrysystem/base/api => ../base/api

require github.com/lib/pq v1.10.9

require google.golang.org/grpc v1.76.0 // indirect

require (
	golang.org/x/net v0.42.0 // indirect
	golang.org/x/sys v0.35.0 // indirect
	golang.org/x/text v0.27.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20250804133106-a7a43d27e69b // indirect
	google.golang.org/protobuf v1.36.10 // indirect
	industrialregistrysystem/base/api v0.0.0
)
