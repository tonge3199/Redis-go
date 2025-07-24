## Redis is a key-value pair db
one `redis.h/redisDb` structure represents a db.

key space : dict in redisDb stores all k-v in db.

```go
package unkownSofar

type redisDb struct {
  // ...
  dict *dict;
  // ...
}
```
add operation 
  key (string) : value (any Redis object)
