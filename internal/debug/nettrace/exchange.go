package nettrace

// import (
// 	"fmt"
// 	"time"
// )

// type Exchange struct {
// 	Request  Message
// 	Response Message
// }

// func (e *Exchange) Time() time.Time {
// 	return e.Request.Time()
// }

// func (e *Exchange) Span() time.Duration {
// 	return e.Response.Time().Sub(e.Request.Time()) + e.Response.Span()
// }

// func (e *Exchange) Format(w fmt.State, v rune) {

// }

// func (e *Exchange) MarshalJSON() ([]byte, error) {
// 	return []byte(`{}`), nil
// }

// func (e *Exchange) MarshalYAML() (any, error) {
// 	return nil, nil
// }
