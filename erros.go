package controlloop

import "errors"

var KeyNotExist = errors.New("key does't exist")
var AlreadyUpdated = errors.New("resource already updated")
