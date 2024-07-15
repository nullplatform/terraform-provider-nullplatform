package nullplatform

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"
)

const PARAMETER_PATH = "/parameter"

type ParameterValue struct {
	Id            string            `json:"id,omitempty"`
	Nrn           string            `json:"nrn,omitempty"`
	Value         string            `json:"value"` // Can be an empty value
	OriginVersion int               `json:"origin_version,omitempty"`
	Dimensions    map[string]string `json:"dimensions,omitempty"`
	CreatedAt     time.Time         `json:"created_at,omitempty"`
	GeneratedId   string            `json:"generated_id,omitempty"`
}

type Parameter struct {
	Id              int               `json:"id,omitempty"`
	Name            string            `json:"name"`
	Nrn             string            `json:"nrn"`
	Type            string            `json:"type"`
	Encoding        string            `json:"encoding"`
	Variable        string            `json:"variable,omitempty"`
	DestinationPath string            `json:"destination_path,omitempty"`
	Secret          bool              `json:"secret"`
	ReadOnly        bool              `json:"read_only"`
	Values          []*ParameterValue `json:"values,omitempty"`
	VersionID       int               `json:"version_id,omitempty"`
}

type Paging struct {
	Offset int `json:"offset,omitempty"`
	Limit  int `json:"limit,omitempty"`
}

type ParameterList struct {
	Paging  *Paging      `json:"paging,omitempty"`
	Results []*Parameter `json:"results,omitempty"`
}

func (c *NullClient) CreateParameter(param *Parameter, importIfCreated bool) (*Parameter, error) {
	url := fmt.Sprintf("https://%s%s", c.ApiURL, PARAMETER_PATH)

	parameterList, err := c.GetParameterList(param.Nrn)
	if err != nil {
		return nil, err
	}

	paramRes, paramExists := parameterExists(parameterList, param)
	if paramExists && importIfCreated {
		return paramRes, nil
	}

	var buf bytes.Buffer
	err = json.NewEncoder(&buf).Encode(*param)

	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", url, &buf)
	if err != nil {
		return nil, err
	}

	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", c.Token.AccessToken))

	res, err := c.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		if res.StatusCode == http.StatusGatewayTimeout {
			return nil, fmt.Errorf("error creating Parameter, status code: %d, message: Gateway Timeout", res.StatusCode)
		}

		nErr := &NullErrors{}
		dErr := json.NewDecoder(res.Body).Decode(nErr)
		if dErr != nil {
			return nil, fmt.Errorf("error creating Parameter, status code: %d, message: %s", res.StatusCode, dErr)
		}
		return nil, fmt.Errorf("error creating Parameter, status code: %d, message: %s", res.StatusCode, nErr.Message)
	}

	paramRes = &Parameter{}
	derr := json.NewDecoder(res.Body).Decode(paramRes)

	if derr != nil {
		return nil, derr
	}

	return paramRes, nil
}

func (c *NullClient) GetParameter(parameterId string) (*Parameter, error) {
	url := fmt.Sprintf("https://%s%s/%s", c.ApiURL, PARAMETER_PATH, parameterId)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", c.Token.AccessToken))

	res, err := c.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	param := &Parameter{}
	derr := json.NewDecoder(res.Body).Decode(param)

	if derr != nil {
		return nil, derr
	}

	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("error getting Parameter resource, got %d for %s", res.StatusCode, parameterId)
	}

	return param, nil
}

func (c *NullClient) PatchParameter(parameterId string, param *Parameter) error {
	url := fmt.Sprintf("https://%s%s/%s", c.ApiURL, PARAMETER_PATH, parameterId)

	var buf bytes.Buffer
	err := json.NewEncoder(&buf).Encode(*param)

	if err != nil {
		return err
	}

	req, err := http.NewRequest("PATCH", url, &buf)
	if err != nil {
		return err
	}

	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", c.Token.AccessToken))

	res, err := c.Client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if (res.StatusCode != http.StatusOK) && (res.StatusCode != http.StatusNoContent) {
		return fmt.Errorf("error patching Parameter resource, got %d", res.StatusCode)
	}

	return nil
}

func (c *NullClient) DeleteParameter(parameterId string) error {
	url := fmt.Sprintf("https://%s%s/%s", c.ApiURL, PARAMETER_PATH, parameterId)

	req, err := http.NewRequest("DELETE", url, nil)
	if err != nil {
		return err
	}

	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", c.Token.AccessToken))

	res, err := c.Client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if (res.StatusCode != http.StatusOK) && (res.StatusCode != http.StatusNoContent) {
		return fmt.Errorf("error deleting Parameter resource, got %d", res.StatusCode)
	}

	return nil
}

func (c *NullClient) CreateParameterValue(paramId int, paramValue *ParameterValue) (*ParameterValue, error) {
	url := fmt.Sprintf("https://%s%s/%s/value", c.ApiURL, PARAMETER_PATH, strconv.Itoa(paramId))

	var buf bytes.Buffer
	err := json.NewEncoder(&buf).Encode(*paramValue)

	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", url, &buf)
	if err != nil {
		return nil, err
	}

	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", c.Token.AccessToken))

	res, err := c.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		if res.StatusCode == http.StatusGatewayTimeout {
			return nil, fmt.Errorf("error creating Parameter Value, status code: %d, message: Gateway Timeout", res.StatusCode)
		}

		nErr := &NullErrors{}
		if dErr := json.NewDecoder(res.Body).Decode(nErr); dErr != nil {
			return nil, fmt.Errorf("error creating Parameter Value, status code: %d, message: %w", res.StatusCode, dErr)
		}
		return nil, fmt.Errorf("error creating Parameter Value, status code: %d, message: %s", res.StatusCode, nErr.Message)
	}

	paramRes := &ParameterValue{}
	derr := json.NewDecoder(res.Body).Decode(paramRes)

	if derr != nil {
		return nil, derr
	}

	return paramRes, nil
}

func (c *NullClient) DeleteParameterValue(parameterId string, parameterValueId string) error {
	url := fmt.Sprintf("https://%s%s/%s/value/%s", c.ApiURL, PARAMETER_PATH, parameterId, parameterValueId)

	req, err := http.NewRequest("DELETE", url, nil)
	if err != nil {
		return err
	}

	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", c.Token.AccessToken))

	res, err := c.Client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if (res.StatusCode != http.StatusOK) && (res.StatusCode != http.StatusNoContent) {
		return fmt.Errorf("error deleting Parameter resource, got %d", res.StatusCode)
	}

	return nil
}

func (c *NullClient) GetParameterValue(parameterId string, parameterValueId string) (*ParameterValue, error) {
	var parameterValue *ParameterValue

	param, err := c.GetParameter(parameterId)
	if err != nil {
		return nil, fmt.Errorf("Parameter ID %s not found", parameterId)
	}

	for _, item := range param.Values {
		if parameterValueId == generateParameterValueID(item) {
			parameterValue = item
			parameterValue.GeneratedId = parameterValueId
			break
		}
	}

	if parameterValue == nil {
		return nil, fmt.Errorf("parameter Value ID %s not found", parameterValueId)
	}

	return parameterValue, nil
}

func generateParameterValueID(value *ParameterValue) string {
	var concatenatedString string

	// Concatenate all key-value pairs from the map
	for key, value := range value.Dimensions {
		concatenatedString += key + ":" + value + ";"
	}

	concatenatedString += value.Nrn + ";"

	// Hash the concatenated string using SHA-256
	hash := sha256.New()
	hash.Write([]byte(concatenatedString))
	hashBytes := hash.Sum(nil)

	// Convert the hash bytes to a hexadecimal string
	hashString := hex.EncodeToString(hashBytes)

	return hashString
}

func (c *NullClient) GetParameterList(nrn string) (*ParameterList, error) {
	// TODO: Implement pagination
	url := fmt.Sprintf("https://%s%s/?nrn=%s&limit=200", c.ApiURL, PARAMETER_PATH, nrn)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", c.Token.AccessToken))

	res, err := c.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	param := &ParameterList{}
	derr := json.NewDecoder(res.Body).Decode(param)

	if derr != nil {
		return nil, derr
	}

	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("error getting Parameter List, got %d for %s", res.StatusCode, nrn)
	}

	return param, nil
}

func parameterExists(parameterList *ParameterList, param *Parameter) (*Parameter, bool) {
	for _, parameter := range parameterList.Results {
		if parameter.Name == param.Name {
			return parameter, true
		}
	}
	return nil, false
}
