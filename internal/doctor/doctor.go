package doctor

import "fmt"

type Detail struct {
	Name        string `json:"name,omitempty"`
	OK          bool   `json:"ok"`
	Message     string `json:"message"`
	Remediation string `json:"remediation,omitempty"`
}

type Result struct {
	OK     bool     `json:"ok"`
	Checks []Detail `json:"checks"`
}

type Check struct {
	Name string
	Run  func() Detail
}

type Runner struct {
	Checks []Check
}

func (r Runner) Run() Result {
	result := Result{OK: true, Checks: make([]Detail, 0, len(r.Checks))}
	for _, check := range r.Checks {
		detail := runCheck(check)
		if !detail.OK {
			result.OK = false
		}
		result.Checks = append(result.Checks, detail)
	}
	return result
}

func runCheck(check Check) (detail Detail) {
	defer func() {
		if recovered := recover(); recovered != nil {
			detail = Detail{OK: false, Message: fmt.Sprintf("panic: %v", recovered)}
		}
		detail.Name = check.Name
	}()
	if check.Run == nil {
		return Detail{OK: false, Message: "check is not configured"}
	}
	return check.Run()
}

func DetailFromError(err error, remediation string) Detail {
	if err == nil {
		return Detail{OK: true, Message: "ok"}
	}
	return Detail{OK: false, Message: err.Error(), Remediation: remediation}
}
