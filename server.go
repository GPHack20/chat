package torbit

import (
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
)

const chatHelp = `(chatbot): Hello, welcome to the chat room
Commands:
  /help    see this help message again (example: /help)

`

var (
	maxMsgLen          = 10 // @TODO: IMPLEMENT THESE. server should validate
	maxNameLen         = 40
	errMessageTooLong  = errors.New("Messages must be less than 10 characters")
	errUsernameTooLong = errors.New("Usernames cannot be more than 40 charcters")
)

type server struct {
	logger     *log.Logger
	clients    map[string]client
	newConn    chan client
	msgRcv     chan string
	disconnect chan client
}

func (s *server) addClient(c client) {
	s.clients[c.getName()] = c
	c.write(chatHelp)
	s.broadcast("(chatbot): New user " + c.getName() + " has joined.\n")
	go c.read()
}

func (s *server) serve(port string) error {
	server, err := net.Listen("tcp", port)
	if err != nil {
		return err
	}
	s.logger.Println("Server started on ", port)

	// TCP Server
	go func() {
		for {
			conn, err := server.Accept()
			if err != nil {
				s.logger.Println(err.Error())
			}
			s.newConn <- newTCPClient(conn, s)
		}
	}()

	// HTTP Server/Websocket server
	go func() {
		http.HandleFunc("/", homeHandler)
		http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
			newWsClientHandler(s, w, r)
		})
		http.ListenAndServe(":8000", nil)
	}()

	for {
		select {
		case client := <-s.newConn:
			s.addClient(client)

		case msg := <-s.msgRcv:
			s.logger.Print("Message received: ", msg)
			s.broadcast(msg)

		case c := <-s.disconnect:
			s.logger.Printf("Disconnected user %s\n", c.getName())
			s.broadcast(fmt.Sprintf("(chatbot): user %s left the chat\n", c.getName()))
			delete(s.clients, c.getName())
			c.close()
		}
	}
}

// broadcast is the function to use to handle broadcasting to multiple
// rooms n stuff
func (s *server) broadcast(msg string) {
	for _, c := range s.clients {
		err := c.write(msg)
		if err != nil {
			s.logger.Println("Broadcast error: ", err.Error())
		}
	}
}

func ServeTCP(l *log.Logger, port string) error {
	s := &server{
		logger:     l,
		clients:    make(map[string]client),
		newConn:    make(chan client),
		msgRcv:     make(chan string),
		disconnect: make(chan client),
	}
	return s.serve(port)
}

// @TODO: This needs to be a template so the port/ip can be set!
const homeHTML = `<!DOCTYPE html>
<html>
  <head>
    <meta charset="utf-8"/>
    <title>Chat</title>
    <meta name="viewport" content="width=device-width,initial-scale=1"/>
    <link href="https://npmcdn.com/basscss@8.0.0/css/basscss.min.css" rel="stylesheet">
    <style>
      html, body { font-family: "Proxima Nova", Helvetica, Arial, sans-serif }
      .bg-blue { background-color: #07c }
      .white { color: #fff }
      .bold { font-weight: bold }
    </style>
  </head>

  <body class="p2">
    <h1 class="h1">Welcome to the chat room!</h1>
    <form id="form" class="flex">
      <input class="flex-auto px2 py1 bg-white border rounded" type="text" id="msg">
      <input class="px2 py1 bg-blue white bold border rounded" type="submit" value="Send">
    </form>
    <div class="my2" id="box"></div>
  <script src="https://ajax.googleapis.com/ajax/libs/jquery/2.0.3/jquery.min.js"></script>
  <script>
  $(function() {

    var ws = new window.WebSocket("ws://" + document.domain + ":8000/ws");
    var $msg = $("#msg");
    var $box = $("#box");

    ws.onclose = function(e) {
      $box.append("<p class='bold'>Connection closed!</p>");
    };
    ws.onmessage = function(e) {
      $box.append("<p>"+e.data+"</p>");
      increaseUnreadCount();
    };

    ws.onerror = function(e) {
      $box.append("<strong>Error!</strong>")
    };

    $("#form").submit(function(e) {
      e.preventDefault();
      if (!ws) {
          return;
      }
      if (!$msg.val()) {
          return;
      }
      ws.send($msg.val());
      $msg.val("");
    });

    document.addEventListener("visibilitychange", resetUnreadCount);

    function increaseUnreadCount() {
      if (document.hidden === true) {
        var count = parseInt(document.title.match(/\d+/));
        if (!count) {
          document.title = "(1) Chat";
          return;
        }
        document.title = "("+(count+1)+") Chat";
      }
    }

    function resetUnreadCount() {
      if (document.hidden === false) {
        document.title = "Chat";
      }
    }

  });
  </script>
  </body>
</html>
`
