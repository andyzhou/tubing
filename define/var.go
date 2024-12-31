package define

//global para
type (
	//send message para
	SendMsgPara struct {
		//origin message
		Msg []byte

		//filter condition
		GroupId     int64
		ConnIds     []int64
		ReceiverIds []int64
		Tags        []string
		Property    map[string]interface{}
	}
)