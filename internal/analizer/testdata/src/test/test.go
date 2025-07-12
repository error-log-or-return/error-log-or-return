package test

type Logger struct{}

func (l *Logger) ErrorOrDebug(err *error, msg string) {}

type TestStruct struct {
	Field string
	log   *Logger
}

//nolint:error_log_or_return
func (t *TestStruct) IgnoredMethod1() error {
	var err error
	defer t.log.ErrorOrDebug(&err, "")
	return err
}

func (t *TestStruct) IgnoredMethod2() error { //nolint:error_log_or_return
	var err error
	defer t.log.ErrorOrDebug(&err, "")
	return err
}

func (t *TestStruct) InvalidMethod1() error { // want "возвращает error и есть defer с &err"
	var err error
	defer t.log.ErrorOrDebug(&err, "")
	return err
}

func (t *TestStruct) InvalidMethod2() { // want "есть err, нет defer, нет возврата error"
	var err error
	_ = err
}

func (t *TestStruct) InvalidMethod3() { // want "есть err, нет defer, нет возврата error"
	var err error
	defer t.log.ErrorOrDebug(nil, "")
	_ = err
}

func (t *TestStruct) ValidMethod1() { // не возвращает error, но есть defer
	var err error
	defer t.log.ErrorOrDebug(&err, "")
	_ = err
}

func (t *TestStruct) ValidMethod2() error { // возвращает error, но нет defer
	var err error
	return err
}

func (t *TestStruct) ValidMethod3() { // нет err, но есть defer с nil
	defer t.log.ErrorOrDebug(nil, "")
}

func (t *TestStruct) ValidMethod4() {} // нет err, нет defer, нет возврата error

func fn() error { return nil }

func (t *TestStruct) ValidMethod5() { // err объявлена не на уровне функции
	if err := fn(); err != nil {

	}
}
