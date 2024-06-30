<!DOCTYPE html PUBLIC "-//W3C//DTD XHTML 1.0 Transitional//EN" "http://www.w3.org/TR/xhtml1/DTD/xhtml1-transitional.dtd">
<html xmlns="http://www.w3.org/1999/xhtml">
<head>
<meta http-equiv="Content-Type" content="text/html; charset=utf-8" />
<style type="text/css">
.message {
    font-size: 16px;
    width:520px;
    height: 300px;
    overflow-y:auto;
    padding: 2px 2px 2px 2px;
    margin-bottom: 5px;
    margin-right: 5px;
    border: 2px solid #99ccff;
}
</style>
</head>
<body>
<script src="/html/jquery.min.js"></script>
<script src="/html/chat.js"></script>

<!-- live chat div -->
<div class="message" id="log">
欢迎进入直播聊天室..
</div>
<div class="genDiv">
  <input type="text" id="msg" size="51"/>
  <input type="submit" id="sendBtn" value="发送"/>
</div>

<script type="text/javascript">
var chatServerAddr = "localhost:8090/ws";
var chatServerChannel = "test";
var userId = 1;
var userNick = "xxxx";
var userSession = "1234";

$(function() {
  //begin connect chat server
  chat_server_conn(chatServerAddr, chatServerChannel, userSession);

  $("#sendBtn").click(function() {
        chat_message_send("msg");
    });
  $('#msg').keydown(function(e){
        if(e.keyCode==13){
            chat_message_send("msg");
        }
    });

});
</script>

</body>
</html>