//go:build !js

package interpreter

import (
	"fmt"
	"net/url"
	"time"

	"github.com/gorilla/websocket"
)

// === Модуль "вс" (WebSocket) ===
//
// Использование:
//   конст сокет = вс.подключить("wss://stream.binance.com:9443/ws/btcusdt@trade")
//   сокет.при_сообщении(функция(данные) вывод(данные) конец)
//   сокет.отправить("ping")
//   сокет.закрыть()

func registerWebSocketModule() {
	fns := map[string]func(args ...Object) Object{
		"подключить": func(args ...Object) Object {
			if len(args) < 1 {
				return ErrorWithHint(currentCallToken, "подключить требует URL", "вс.подключить(\"wss://...\")")
			}
			urlStr, ok := args[0].(*String)
			if !ok {
				return ErrorWithHint(currentCallToken, "URL должен быть строкой", "")
			}

			// Валидация URL
			u, err := url.Parse(urlStr.Value)
			if err != nil || (u.Scheme != "ws" && u.Scheme != "wss") {
				return ErrorWithHint(currentCallToken, "неверный WebSocket URL (нужен ws:// или wss://)", "")
			}

			conn, _, err := websocket.DefaultDialer.Dial(urlStr.Value, nil)
			if err != nil {
				return ErrorWithHint(currentCallToken, "не удалось подключиться: "+err.Error(), "Проверьте URL и доступность сервера.")
			}

			return newWSConnection(conn)
		},
	}
	stdModules["вс"] = makeHashFromBuiltins(fns)
}

func newWSConnection(conn *websocket.Conn) *Hash {
	h := NewHash()
	closed := false

	// отправить(текст) — отправить текстовое сообщение
	h.Set(&String{Value: "отправить"}, &Builtin{Fn: func(args ...Object) Object {
		if closed {
			return ErrorWithHint(currentCallToken, "соединение закрыто", "")
		}
		if len(args) != 1 {
			return ErrorWithHint(currentCallToken, "отправить(текст)", "")
		}
		msg, ok := args[0].(*String)
		if !ok {
			return ErrorWithHint(currentCallToken, "аргумент должен быть строкой", "")
		}
		err := conn.WriteMessage(websocket.TextMessage, []byte(msg.Value))
		if err != nil {
			return ErrorWithHint(currentCallToken, "ошибка отправки: "+err.Error(), "")
		}
		return NULL
	}})

	// отправить_json(объект) — сериализует и отправляет
	h.Set(&String{Value: "отправить_json"}, &Builtin{Fn: func(args ...Object) Object {
		if closed {
			return ErrorWithHint(currentCallToken, "соединение закрыто", "")
		}
		if len(args) != 1 {
			return ErrorWithHint(currentCallToken, "отправить_json(объект)", "")
		}
		err := conn.WriteJSON(args[0].Inspect())
		if err != nil {
			return ErrorWithHint(currentCallToken, "ошибка отправки: "+err.Error(), "")
		}
		return NULL
	}})

	// получить() — блокирующее чтение одного сообщения
	h.Set(&String{Value: "получить"}, &Builtin{Fn: func(args ...Object) Object {
		if closed {
			return NULL
		}
		_, message, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
				closed = true
				return NULL
			}
			return ErrorWithHint(currentCallToken, "ошибка чтения: "+err.Error(), "")
		}
		return &String{Value: string(message)}
	}})

	// слушать(количество, обработчик) — получить N сообщений, вызвать функцию для каждого
	h.Set(&String{Value: "слушать"}, &Builtin{Fn: func(args ...Object) Object {
		if len(args) < 2 {
			return ErrorWithHint(currentCallToken, "слушать(количество, функция)", "слушать(10, функция(сообщение) ... конец)")
		}
		count, ok := args[0].(*Integer)
		if !ok {
			return ErrorWithHint(currentCallToken, "первый аргумент — количество сообщений", "")
		}
		fn := args[1]
		if fn.Type() != "FUNCTION" && fn.Type() != "BUILTIN" {
			return ErrorWithHint(currentCallToken, "второй аргумент — функция-обработчик", "")
		}

		for i := int64(0); i < count.Value; i++ {
			if closed {
				break
			}
			_, message, err := conn.ReadMessage()
			if err != nil {
				if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
					closed = true
					break
				}
				return ErrorWithHint(currentCallToken, "ошибка чтения: "+err.Error(), "")
			}
			result := ApplyFunctionCallback(fn, []Object{&String{Value: string(message)}})
			if isError(result) {
				return result
			}
			// Если обработчик вернул нет — прекращаем
			if result == FALSE {
				break
			}
		}
		return NULL
	}})

	// таймаут(секунды) — установить таймаут чтения
	h.Set(&String{Value: "таймаут"}, &Builtin{Fn: func(args ...Object) Object {
		if len(args) != 1 {
			return ErrorWithHint(currentCallToken, "таймаут(секунды)", "")
		}
		var seconds float64
		switch v := args[0].(type) {
		case *Integer:
			seconds = float64(v.Value)
		case *Float:
			seconds = v.Value
		default:
			return ErrorWithHint(currentCallToken, "аргумент должен быть числом", "")
		}
		conn.SetReadDeadline(time.Now().Add(time.Duration(seconds * float64(time.Second))))
		return NULL
	}})

	// закрыть()
	h.Set(&String{Value: "закрыть"}, &Builtin{Fn: func(args ...Object) Object {
		if !closed {
			conn.WriteMessage(websocket.CloseMessage,
				websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
			conn.Close()
			closed = true
		}
		return NULL
	}})

	// открыт?() — проверить состояние
	h.Set(&String{Value: "открыт?"}, &Builtin{Fn: func(args ...Object) Object {
		return nativeBoolToBoolean(!closed)
	}})

	// url
	h.Set(&String{Value: "__url__"}, &String{Value: fmt.Sprintf("%s", conn.RemoteAddr())})

	return h
}
