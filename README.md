# tubing
tubing is a http web socket proxy library, base one gin.

# base rule
- create base ws router
- mark different conn property for diff application

# code introduce
- server.go
  - used for server side
- client.go
  - used for client side
  
# how to use
- pls see `example` sub dir, this is a simple chat demo.

## testing
go test -v -run="Client"
go test -bench="Client"
go test -bench="Client" -benchmem -benchtime=10s