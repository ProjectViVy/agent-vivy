module example.com/vivy/plugins/feishu

go 1.26.4

replace agent-vivy => ../..

require (
	agent-vivy v0.0.0
	github.com/larksuite/oapi-sdk-go/v3 v3.11.0
)

require (
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/gorilla/websocket v1.5.3 // indirect
)
