//////////////////////
//js for chat
//web socket support
//////////////////////
var userId = 0;
var userNick = "";
var wsConn = null;

//sign up new channel
function chat_server_sign_up(serverAddr) {
    var name = $("#name");
    var introduce = $("#introduce");
    var password = $("#password");

    if(typeof(name) == "undefined" || name == "") {
        return;
    }

    if(typeof(password) == "undefined" || password == "") {
        return;
    }

    if(!confirm('确定注册么?')) {
        return;
    }

    var reqUrl = serverAddr + "/signUp";
    var data = {
        name:name,
        introduce:introduce,
        password:password,
    };
    sendAjaxReqWithCB(reqUrl, data, null);
}

//login chat server
function chat_server_login(userId, userNick) {
    if(typeof(userId) == "undefined" || userId == "") {
        return;
    }
    if(typeof(userNick) == "undefined" || userNick == "") {
        return;
    }
    if(typeof(wsConn) == "undefined" || wsConn == null) {
        return;
    }
    //create json object
    var loginObj = new Object();
    loginObj.id = userId
    loginObj.nick = userNick

    var genOpt = new Object();
    genOpt.kind = "login"
    genOpt.jsonObj = loginObj

    var jsonStr = JSON.stringify(genOpt);

    //send login object to server
    wsConn.send(jsonStr);
}


//send message to server
function chat_message_send(messageDiv) {
    var message = $("#"+messageDiv);
    var messageInfo = message.val();
    if(typeof(messageInfo) == "undefined" || messageInfo == "") {
        return;
    }
    if(typeof(wsConn) == "undefined" || wsConn == null) {
        return;
    }
    //create json object
    var chatObj = new Object();
    chatObj.message = messageInfo;

    var genOpt = new Object();
    genOpt.kind = "chat"
    genOpt.jsonObj = chatObj

    var jsonStr = JSON.stringify(genOpt);
    wsConn.send(jsonStr);
    message.val("");
}

//receive message from chat server
function chat_server_message(message) {
    if(typeof(message) == "undefined" || message == null || message == "") {
        return;
    }
    //check json object
    var isJson = isJsonStr(message)
    var finalMessage = "";
    if(isJson) {
        console.log("message:"+message)
        //try parse json data
        var dataObj = JSON.parse(message);
        if(typeof(dataObj) != "undefined" && dataObj != null) {
            var message = dataObj.message;
            var sender = dataObj.sender;
            finalMessage = sender + ':' + message;
        }else{
            finalMessage = message
        }
    }else{
        //not json data, just display message
        finalMessage = message
    }
    chat_message_append($("<div/>").text(finalMessage));
}


//connect server
function chat_server_conn (serverAddr, channel, session) {
    if(typeof(serverAddr) == "undefined" || serverAddr == "") {
        return false;
    }
    if(typeof(channel) == "undefined" || channel == "") {
        return false;
    }

    if(!window["WebSocket"]){
        chat_message_append($("<div><b>Your browser does not support WebSockets.</b></div>"))
        return false;
    }

    //init web socket
   // var chatAddr = "ws://" + serverAddr + "/chat/" + channel;
    //var chatAddr = "ws://adam.abc.com:6600/chat/test"
    var chatAddr = "ws://" + serverAddr + "?session=" + session;
    wsConn = new WebSocket(chatAddr);

    //connect close
    wsConn.onclose = function(evt) {
        chat_message_append($("<div>聊天链接关闭..</div>"));
    }

    //connect success
    wsConn.onopen = function(evt) {
        chat_message_append($("<div>链接聊天服务器成功..</div>"));
        //try login to chat server
        chat_server_login(userId, userNick);
    }

    //received message
    wsConn.onmessage = function(evt) {
        //alert('data:' + evt.data);
       //var dataObj = JSON.parse(evt.data);
        //process message
        chat_server_message(evt.data);
    }

    return true;
}


//append chat into message list
function chat_message_append(message) {
    if(typeof(message) == "undefined" || message == "") {
        return;
    }
    var displayDiv = $("#log");
    var d = displayDiv[0];
    var doScroll = d.scrollTop == d.scrollHeight - d.clientHeight;

    //append message to div
    message.appendTo(displayDiv);

    //check scroll
    if (doScroll) {
        d.scrollTop = d.scrollHeight - d.clientHeight;
    }
}

//json string check
function isJsonStr(str) {
    if (typeof str == 'string') {
        try {
            var obj=JSON.parse(str);
            if(typeof obj == 'object' && obj ){
                return true;
            }else{
                return false;
            }

        } catch(e) {
            return false;
        }
    }else if (typeof str == 'object'  && str) {
        return true;
    }
}

//////////////////////////
//send ajax request
//////////////////////////
function sendAjaxReqWithCB(reqUrl, data, cbFunc) {
    if(typeof(reqUrl) == "undefined" || reqUrl == "") {
        return
    }
    //send ajax request
    $.ajax({
        type: "Post",
        url: reqUrl,
        data: data,
        async : true,
        //dataType : "json",
        success: function(data){
            if(typeof(data) == undefined) {
                alert('无效的返回数据');
                return
            }
            //get resp of json
            var errCode = data.errCode
            var errMsg = data.errMsg
            if(errCode != 1000) {
                alert("处理失败, 错误:" + errMsg);
                return
            }
            if(typeof(cbFunc) != "undefined" && cbFunc != null) {
                cbFunc(data.jsonObj);
            }
        },
        error: function(err) {
            alert(err);
        }
    });
}