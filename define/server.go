package define

//websocket
const (
	WebSocketRoot = "/ws"
	WebSocketBufferSize = 1024 * 5
)

//param
const (
	QueryParaOfContentType = "type"
	QueryParaOfSession = "session"
)

//request method
const (
	ReqMethodOfGet = "GET"
	ReqMethodOfPost = "POST"
)

//request path
const (
	AnyPath = "any"
	ModulePath = "module"
	SubModulePath = "sub"
)

//default value
const (
	AppName = "tubing"
	AppHttpPort = 8686
	AppTcpPort = 8585
	ReqTimeOutSeconds = 10
	ReqMaxConn = 256
)

const (
	HeaderOfContentType = "Content-Type"
	ContentTypeOfJson = "application/json"
	ContentTypeOfOctet = "application/octet"
)