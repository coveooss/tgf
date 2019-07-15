// Code generated by moq; DO NOT EDIT.
// github.com/matryer/moq

package main

import (
	"sync"
	"time"
)

var (
	lockRunnerUpdaterMockDebug            sync.RWMutex
	lockRunnerUpdaterMockDoUpdate         sync.RWMutex
	lockRunnerUpdaterMockGetLastRefresh   sync.RWMutex
	lockRunnerUpdaterMockGetUpdateVersion sync.RWMutex
	lockRunnerUpdaterMockRestart          sync.RWMutex
	lockRunnerUpdaterMockRun              sync.RWMutex
	lockRunnerUpdaterMockSetLastRefresh   sync.RWMutex
	lockRunnerUpdaterMockShouldUpdate     sync.RWMutex
)

// Ensure, that RunnerUpdaterMock does implement RunnerUpdater.
// If this is not the case, regenerate this file with moq.
var _ RunnerUpdater = &RunnerUpdaterMock{}

// RunnerUpdaterMock is a mock implementation of RunnerUpdater.
//
//     func TestSomethingThatUsesRunnerUpdater(t *testing.T) {
//
//         // make and configure a mocked RunnerUpdater
//         mockedRunnerUpdater := &RunnerUpdaterMock{
//             DebugFunc: func(format string, args ...interface{})  {
// 	               panic("mock out the Debug method")
//             },
//             DoUpdateFunc: func(url string) error {
// 	               panic("mock out the DoUpdate method")
//             },
//             GetLastRefreshFunc: func(file string) time.Duration {
// 	               panic("mock out the GetLastRefresh method")
//             },
//             GetUpdateVersionFunc: func() (string, error) {
// 	               panic("mock out the GetUpdateVersion method")
//             },
//             RestartFunc: func() int {
// 	               panic("mock out the Restart method")
//             },
//             RunFunc: func() int {
// 	               panic("mock out the Run method")
//             },
//             SetLastRefreshFunc: func(file string)  {
// 	               panic("mock out the SetLastRefresh method")
//             },
//             ShouldUpdateFunc: func() bool {
// 	               panic("mock out the ShouldUpdate method")
//             },
//         }
//
//         // use mockedRunnerUpdater in code that requires RunnerUpdater
//         // and then make assertions.
//
//     }
type RunnerUpdaterMock struct {
	// DebugFunc mocks the Debug method.
	DebugFunc func(format string, args ...interface{})

	// DoUpdateFunc mocks the DoUpdate method.
	DoUpdateFunc func(url string) error

	// GetLastRefreshFunc mocks the GetLastRefresh method.
	GetLastRefreshFunc func(file string) time.Duration

	// GetUpdateVersionFunc mocks the GetUpdateVersion method.
	GetUpdateVersionFunc func() (string, error)

	// RestartFunc mocks the Restart method.
	RestartFunc func() int

	// RunFunc mocks the Run method.
	RunFunc func() int

	// SetLastRefreshFunc mocks the SetLastRefresh method.
	SetLastRefreshFunc func(file string)

	// ShouldUpdateFunc mocks the ShouldUpdate method.
	ShouldUpdateFunc func() bool

	// calls tracks calls to the methods.
	calls struct {
		// Debug holds details about calls to the Debug method.
		Debug []struct {
			// Format is the format argument value.
			Format string
			// Args is the args argument value.
			Args []interface{}
		}
		// DoUpdate holds details about calls to the DoUpdate method.
		DoUpdate []struct {
			// URL is the url argument value.
			URL string
		}
		// GetLastRefresh holds details about calls to the GetLastRefresh method.
		GetLastRefresh []struct {
			// File is the file argument value.
			File string
		}
		// GetUpdateVersion holds details about calls to the GetUpdateVersion method.
		GetUpdateVersion []struct {
		}
		// Restart holds details about calls to the Restart method.
		Restart []struct {
		}
		// Run holds details about calls to the Run method.
		Run []struct {
		}
		// SetLastRefresh holds details about calls to the SetLastRefresh method.
		SetLastRefresh []struct {
			// File is the file argument value.
			File string
		}
		// ShouldUpdate holds details about calls to the ShouldUpdate method.
		ShouldUpdate []struct {
		}
	}
}

// Debug calls DebugFunc.
func (mock *RunnerUpdaterMock) Debug(format string, args ...interface{}) {
	if mock.DebugFunc == nil {
		panic("RunnerUpdaterMock.DebugFunc: method is nil but RunnerUpdater.Debug was just called")
	}
	callInfo := struct {
		Format string
		Args   []interface{}
	}{
		Format: format,
		Args:   args,
	}
	lockRunnerUpdaterMockDebug.Lock()
	mock.calls.Debug = append(mock.calls.Debug, callInfo)
	lockRunnerUpdaterMockDebug.Unlock()
	mock.DebugFunc(format, args...)
}

// DebugCalls gets all the calls that were made to Debug.
// Check the length with:
//     len(mockedRunnerUpdater.DebugCalls())
func (mock *RunnerUpdaterMock) DebugCalls() []struct {
	Format string
	Args   []interface{}
} {
	var calls []struct {
		Format string
		Args   []interface{}
	}
	lockRunnerUpdaterMockDebug.RLock()
	calls = mock.calls.Debug
	lockRunnerUpdaterMockDebug.RUnlock()
	return calls
}

// DoUpdate calls DoUpdateFunc.
func (mock *RunnerUpdaterMock) DoUpdate(url string) error {
	if mock.DoUpdateFunc == nil {
		panic("RunnerUpdaterMock.DoUpdateFunc: method is nil but RunnerUpdater.DoUpdate was just called")
	}
	callInfo := struct {
		URL string
	}{
		URL: url,
	}
	lockRunnerUpdaterMockDoUpdate.Lock()
	mock.calls.DoUpdate = append(mock.calls.DoUpdate, callInfo)
	lockRunnerUpdaterMockDoUpdate.Unlock()
	return mock.DoUpdateFunc(url)
}

// DoUpdateCalls gets all the calls that were made to DoUpdate.
// Check the length with:
//     len(mockedRunnerUpdater.DoUpdateCalls())
func (mock *RunnerUpdaterMock) DoUpdateCalls() []struct {
	URL string
} {
	var calls []struct {
		URL string
	}
	lockRunnerUpdaterMockDoUpdate.RLock()
	calls = mock.calls.DoUpdate
	lockRunnerUpdaterMockDoUpdate.RUnlock()
	return calls
}

// GetLastRefresh calls GetLastRefreshFunc.
func (mock *RunnerUpdaterMock) GetLastRefresh(file string) time.Duration {
	if mock.GetLastRefreshFunc == nil {
		panic("RunnerUpdaterMock.GetLastRefreshFunc: method is nil but RunnerUpdater.GetLastRefresh was just called")
	}
	callInfo := struct {
		File string
	}{
		File: file,
	}
	lockRunnerUpdaterMockGetLastRefresh.Lock()
	mock.calls.GetLastRefresh = append(mock.calls.GetLastRefresh, callInfo)
	lockRunnerUpdaterMockGetLastRefresh.Unlock()
	return mock.GetLastRefreshFunc(file)
}

// GetLastRefreshCalls gets all the calls that were made to GetLastRefresh.
// Check the length with:
//     len(mockedRunnerUpdater.GetLastRefreshCalls())
func (mock *RunnerUpdaterMock) GetLastRefreshCalls() []struct {
	File string
} {
	var calls []struct {
		File string
	}
	lockRunnerUpdaterMockGetLastRefresh.RLock()
	calls = mock.calls.GetLastRefresh
	lockRunnerUpdaterMockGetLastRefresh.RUnlock()
	return calls
}

// GetUpdateVersion calls GetUpdateVersionFunc.
func (mock *RunnerUpdaterMock) GetUpdateVersion() (string, error) {
	if mock.GetUpdateVersionFunc == nil {
		panic("RunnerUpdaterMock.GetUpdateVersionFunc: method is nil but RunnerUpdater.GetUpdateVersion was just called")
	}
	callInfo := struct {
	}{}
	lockRunnerUpdaterMockGetUpdateVersion.Lock()
	mock.calls.GetUpdateVersion = append(mock.calls.GetUpdateVersion, callInfo)
	lockRunnerUpdaterMockGetUpdateVersion.Unlock()
	return mock.GetUpdateVersionFunc()
}

// GetUpdateVersionCalls gets all the calls that were made to GetUpdateVersion.
// Check the length with:
//     len(mockedRunnerUpdater.GetUpdateVersionCalls())
func (mock *RunnerUpdaterMock) GetUpdateVersionCalls() []struct {
} {
	var calls []struct {
	}
	lockRunnerUpdaterMockGetUpdateVersion.RLock()
	calls = mock.calls.GetUpdateVersion
	lockRunnerUpdaterMockGetUpdateVersion.RUnlock()
	return calls
}

// Restart calls RestartFunc.
func (mock *RunnerUpdaterMock) Restart() int {
	if mock.RestartFunc == nil {
		panic("RunnerUpdaterMock.RestartFunc: method is nil but RunnerUpdater.Restart was just called")
	}
	callInfo := struct {
	}{}
	lockRunnerUpdaterMockRestart.Lock()
	mock.calls.Restart = append(mock.calls.Restart, callInfo)
	lockRunnerUpdaterMockRestart.Unlock()
	return mock.RestartFunc()
}

// RestartCalls gets all the calls that were made to Restart.
// Check the length with:
//     len(mockedRunnerUpdater.RestartCalls())
func (mock *RunnerUpdaterMock) RestartCalls() []struct {
} {
	var calls []struct {
	}
	lockRunnerUpdaterMockRestart.RLock()
	calls = mock.calls.Restart
	lockRunnerUpdaterMockRestart.RUnlock()
	return calls
}

// Run calls RunFunc.
func (mock *RunnerUpdaterMock) Run() int {
	if mock.RunFunc == nil {
		panic("RunnerUpdaterMock.RunFunc: method is nil but RunnerUpdater.Run was just called")
	}
	callInfo := struct {
	}{}
	lockRunnerUpdaterMockRun.Lock()
	mock.calls.Run = append(mock.calls.Run, callInfo)
	lockRunnerUpdaterMockRun.Unlock()
	return mock.RunFunc()
}

// RunCalls gets all the calls that were made to Run.
// Check the length with:
//     len(mockedRunnerUpdater.RunCalls())
func (mock *RunnerUpdaterMock) RunCalls() []struct {
} {
	var calls []struct {
	}
	lockRunnerUpdaterMockRun.RLock()
	calls = mock.calls.Run
	lockRunnerUpdaterMockRun.RUnlock()
	return calls
}

// SetLastRefresh calls SetLastRefreshFunc.
func (mock *RunnerUpdaterMock) SetLastRefresh(file string) {
	if mock.SetLastRefreshFunc == nil {
		panic("RunnerUpdaterMock.SetLastRefreshFunc: method is nil but RunnerUpdater.SetLastRefresh was just called")
	}
	callInfo := struct {
		File string
	}{
		File: file,
	}
	lockRunnerUpdaterMockSetLastRefresh.Lock()
	mock.calls.SetLastRefresh = append(mock.calls.SetLastRefresh, callInfo)
	lockRunnerUpdaterMockSetLastRefresh.Unlock()
	mock.SetLastRefreshFunc(file)
}

// SetLastRefreshCalls gets all the calls that were made to SetLastRefresh.
// Check the length with:
//     len(mockedRunnerUpdater.SetLastRefreshCalls())
func (mock *RunnerUpdaterMock) SetLastRefreshCalls() []struct {
	File string
} {
	var calls []struct {
		File string
	}
	lockRunnerUpdaterMockSetLastRefresh.RLock()
	calls = mock.calls.SetLastRefresh
	lockRunnerUpdaterMockSetLastRefresh.RUnlock()
	return calls
}

// ShouldUpdate calls ShouldUpdateFunc.
func (mock *RunnerUpdaterMock) ShouldUpdate() bool {
	if mock.ShouldUpdateFunc == nil {
		panic("RunnerUpdaterMock.ShouldUpdateFunc: method is nil but RunnerUpdater.ShouldUpdate was just called")
	}
	callInfo := struct {
	}{}
	lockRunnerUpdaterMockShouldUpdate.Lock()
	mock.calls.ShouldUpdate = append(mock.calls.ShouldUpdate, callInfo)
	lockRunnerUpdaterMockShouldUpdate.Unlock()
	return mock.ShouldUpdateFunc()
}

// ShouldUpdateCalls gets all the calls that were made to ShouldUpdate.
// Check the length with:
//     len(mockedRunnerUpdater.ShouldUpdateCalls())
func (mock *RunnerUpdaterMock) ShouldUpdateCalls() []struct {
} {
	var calls []struct {
	}
	lockRunnerUpdaterMockShouldUpdate.RLock()
	calls = mock.calls.ShouldUpdate
	lockRunnerUpdaterMockShouldUpdate.RUnlock()
	return calls
}
