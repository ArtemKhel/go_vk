Задание состоит из 2х частей.

### 1 
В первой части требуется реализовать пакет subpub. В этой части задания нужно написать простую шину событий, работающую по принципу Publisher-Subscriber.

Требования к шине:
 - На один subject может подписываться (и отписываться) множество подписчиков.
 - Один медленный подписчик не должен тормозить остальных.
 - Нельзя терять порядок сообщений (FIFO очередь).
 - Метод Close должен учитывать переданный контекст. Если он отменен - выходим сразу, работающие хендлеры оставляем работать.
 - Горутины (если они будут) течь не должны.

Ниже представлен API пакета subpub.
```go
package subpub

import "context"

// MessageHandler is a callback function that processes messages delivered to subscribers.
type MessageHandler func(msg interface{})

type Subscription interface {
	// Unsubscribe will remove interest in the current subject subscription is for.
	Unsubscribe()
}

type SubPub interface {
	// Subscribe creates an asynchronous queue subscriber on the given subject.
	Subscribe(subject string, cb MessageHandler) (Subscription, error)

	// Publish publishes the msg argument to the given subject.
	Publish(subject string, msg interface{}) error

	// Close will shut down sub-pub system.
	// May be blocked by data delivery until the context is canceled.
	Close(ctx context.Context) error
}

func NewSubPub() SubPub {
    panic("Implement me")
}
```

К заданию очень рекомендуется написать unit-тесты.


### 2 
Во второй части задания требуется с использованием пакета subpub из 1 части реализовать сервис подписок. Сервис работает по gRPC. Есть возможность подписаться на события по ключу и опубликовать события по ключу для всех подписчиков.

Protobuf-схема gRPC сервиса:
```protobuf
syntax = "proto3";

package grpc;

import "google/protobuf/empty.proto";

service PubSub {
  // Подписка (сервер отправляет поток событий)
  rpc Subscribe(SubscribeRequest) returns (stream Event);

  // Публикация (классический запрос-ответ)
  rpc Publish(PublishRequest) returns (google.protobuf.Empty);
}

message SubscribeRequest {
  string key = 1;
}

message PublishRequest {
  string key = 1;
  string data = 2;
}

message Event {
  string data = 1;
}
```

Также пользуйся стандартными статус-кодами gRPC из пакетов `google.golang.org/grpc/status` и `google.golang.org/grpc/codes` в качестве критериев успешности и неуспешности запросов к сервису. 

Что еще ожидается в решении:
 - Обязательно должно быть описание того, как работает сервис и как его собирать.
 - У сервиса должен быть свой конфиг, куда можно прописать порты и прочие параметры (на ваше усмотрение).
 - Логирование.

Приветствуется использование известных паттернов при разработке микросервисов на Go (например, dependency injection, graceful shutdown и пр.). Если таковые будут использоваться, то просьба упомянуть его в описании решения.

Решение ожидается в качестве ссылки на репозиторий GitHub, ссылку вставь в поле ответа к этому заданию.

Чтобы проверяющие могли поверить решение нужно, чтобы ваш репозиторий был публичным!

