package main

import (
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
)

func respCodeToErrorMsg(resp *http.Response, expectedCode int) string {
	if resp.StatusCode == 401 {
		return fmt.Sprint("Unauthorized, please log in first.")
	}
	return fmt.Sprintf("Unexpected return code from API, was=%d, expected=%d", resp.StatusCode, expectedCode)
}

func extractProjectFromResponse(resp *http.Response, expectedCode int, listExpected bool) ([]Project, error) {
	if resp.StatusCode != expectedCode {
		Log.Debugf("resp.Code=%#v / expected=%d", resp.StatusCode, expectedCode)
		return nil, errors.New(respCodeToErrorMsg(resp, expectedCode))
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		Log.Debugf("err=%#v", err)
		return nil, err
	}
	Log.Debugf("body=%v", string(body))

	if resp.StatusCode != expectedCode {
		return nil, errors.New(fmt.Sprintf("Failed with the wrong code: %v. (expected %v)\n", resp.StatusCode, expectedCode))
	}

	if listExpected {
		return extractProjectsFromBody(body)
	}

	return extractProjectFromBody(body)
}

func extractAssetFromResponse(resp *http.Response, expectedCode int, listExpected bool) ([]Asset, error) {
	if resp.StatusCode != expectedCode {
		Log.Debugf("resp.Code=%#v / expected=%d", resp.StatusCode, expectedCode)
		return nil, errors.New(respCodeToErrorMsg(resp, expectedCode))
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		Log.Debugf("err=%#v", err)
		return nil, err
	}
	Log.Debugf("body=%v", string(body))

	if resp.StatusCode != expectedCode {
		return nil, errors.New(fmt.Sprintf("Failed with the wrong code: %v. (expected %v)\n", resp.StatusCode, expectedCode))
	}

	if listExpected {
		return extractAssetsFromBody(body)
	}

	return extractAssetFromBody(body)
}

func extractJobFromResponse(resp *http.Response, expectedCode int, listExpected bool) ([]Job, error) {
	if resp.StatusCode != expectedCode {
		Log.Debugf("resp.Code=%#v / expected=%d", resp.StatusCode, expectedCode)
		return nil, errors.New(respCodeToErrorMsg(resp, expectedCode))
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		Log.Debugf("err=%#v", err)
		return nil, err
	}
	Log.Debugf("body=%v", string(body))

	if resp.StatusCode != expectedCode {
		return nil, errors.New(fmt.Sprintf("Failed with the wrong code: %v. (expected %v)\n", resp.StatusCode, expectedCode))
	}

	if listExpected {
		return extractJobsFromBody(body)
	}

	return extractJobFromBody(body)
}
