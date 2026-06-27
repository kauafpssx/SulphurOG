package domain

import "errors"

// ErrStorageFull é retornado quando o bucket Supabase atingiu o limite de armazenamento.
// Não deve marcar o arquivo como falho — deve pausar e retryar.
var ErrStorageFull = errors.New("storage: bucket quota exceeded")
