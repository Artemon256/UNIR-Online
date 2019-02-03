package logger

import (
  "fmt"
  "log"
  "os"
)

type Reporter struct {
  logFile *os.File
}

func (r *Reporter) Init(filename string) error {
  var err error
  r.logFile, err = os.Create(filename)
  return err
}

func (r *Reporter) Log(s string) {
  fmt.Fprint(r.logFile, s, "\n")
  log.Print(s)
}

func (r *Reporter) Finish() {
  r.logFile.Close()
}
