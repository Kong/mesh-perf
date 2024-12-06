// nolint:all  // code here is extracted and revised from github.com/gruntwork-io/terratest
package silent_kubectl

import (
	"github.com/gruntwork-io/terratest/modules/logger"
	"github.com/gruntwork-io/terratest/modules/retry"
	"github.com/gruntwork-io/terratest/modules/testing"
	"time"
)

// DoWithRetryE runs the specified action. If it returns a string, return that string. If it returns a FatalError, return that error
// immediately. If it returns any other type of error, sleep for sleepBetweenRetries and try again, up to a maximum of
// maxRetries retries. If maxRetries is exceeded, return a MaxRetriesExceeded error.
func DoWithRetryE(t testing.TestingT, actionDescription string, maxRetries int, sleepBetweenRetries time.Duration, action func() (string, error)) (string, error) {
	out, err := DoWithRetryInterfaceE(t, actionDescription, maxRetries, sleepBetweenRetries, func() (interface{}, error) { return action() })
	return out.(string), err
}

// DoWithRetryInterfaceE runs the specified action. If it returns a value, return that value. If it returns a FatalError, return that error
// immediately. If it returns any other type of error, sleep for sleepBetweenRetries and try again, up to a maximum of
// maxRetries retries. If maxRetries is exceeded, return a MaxRetriesExceeded error.
func DoWithRetryInterfaceE(t testing.TestingT, actionDescription string, maxRetries int, sleepBetweenRetries time.Duration, action func() (interface{}, error)) (interface{}, error) {
	var output interface{}
	var err error

	for i := 0; i <= maxRetries; i++ {
		output, err = action()
		if err == nil {
			return output, nil
		}

		if _, isFatalErr := err.(retry.FatalError); isFatalErr {
			logger.Logf(t, "Returning due to fatal error: %v", err)
			return output, err
		}

		logger.Logf(t, "%s returned an error: %s. Sleeping for %s and will try again.", actionDescription, err.Error(), sleepBetweenRetries)
		time.Sleep(sleepBetweenRetries)
	}

	return output, retry.MaxRetriesExceeded{Description: actionDescription, MaxRetries: maxRetries}
}
