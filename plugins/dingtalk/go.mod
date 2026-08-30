module example.com/vivy/plugins/dingtalk

go 1.26.4

replace agent-vivy => ../..

require (
	agent-vivy v0.0.0
	github.com/open-dingtalk/dingtalk-stream-sdk-go v0.9.1
)

require (
	github.com/google/uuid v1.6.0 // indirect
	github.com/gorilla/websocket v1.5.3 // indirect
)
