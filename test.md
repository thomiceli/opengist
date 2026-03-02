---
description: Testing handler and middleware
slug: /testing
sidebar_position: 13
---

# Testing

## Testing Handler

`GET` `/users/:id`

Handler below retrieves user by id from the database. If user is not found it returns
`404` error with a message.

### CreateUser

`POST` `/users`

- Accepts JSON payload
- On success `201 - Created`
- On error `500 - Internal Server Error`

### GetUser

`GET` `/users/:email`

- On success `200 - OK`
- On error `404 - Not Found` if user is not found otherwise `500 - Internal Server Error`

`handler.go`

```go
package handler

import (
	"net/http"

	"github.com/labstack/echo/v5"
)

type (
	User struct {
		Name  string `json:"name" form:"name"`
		Email string `json:"email" form:"email"`
	}
	handler struct {
		db map[string]*User
	}
)

func (h *handler) createUser(c *echo.Context) error {
	u := new(User)
	if err := c.Bind(u); err != nil {
		return err
	}
	return c.JSON(http.StatusCreated, u)
}

func (h *handler) getUser(c *echo.Context) error {
	email := c.Param("email")
	user := h.db[email]
	if user == nil {
		return echo.NewHTTPError(http.StatusNotFound, "user not found")
	}
	return c.JSON(http.StatusOK, user)
}
```

`handler_test.go`

```go
package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/echotest"
	"github.com/stretchr/testify/assert"
)

var (
	mockDB = map[string]*User{
		"jon@labstack.com": &User{"Jon Snow", "jon@labstack.com"},
	}
	userJSON = `{"name":"Jon Snow","email":"jon@labstack.com"}`
)

func TestCreateUser(t *testing.T) {
	// Setup
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(userJSON))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)

	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	h := &controller{mockDB}

	// Assertions
	if assert.NoError(t, h.createUser(c)) {
		assert.Equal(t, http.StatusCreated, rec.Code)
		assert.Equal(t, userJSON, rec.Body.String())
	}
}

// Same test as above but using `echotest` package helpers
func TestCreateUserWithEchoTest(t *testing.T) {
	c, rec := echotest.ContextConfig{
		Headers: map[string][]string{
			echo.HeaderContentType: {echo.MIMEApplicationJSON},
		},
		JSONBody: []byte(`{"name":"Jon Snow","email":"jon@labstack.com"}`),
	}.ToContextRecorder(t)

	h := &controller{mockDB}

	// Assertions
	if assert.NoError(t, h.createUser(c)) {
		assert.Equal(t, http.StatusCreated, rec.Code)
		assert.Equal(t, userJSON+"\n", rec.Body.String())
	}
}

// Same test as above but even shorter
func TestCreateUserWithEchoTest2(t *testing.T) {
	h := &controller{mockDB}

	rec := echotest.ContextConfig{
		Headers: map[string][]string{
			echo.HeaderContentType: {echo.MIMEApplicationJSON},
		},
		JSONBody: []byte(`{"name":"Jon Snow","email":"jon@labstack.com"}`),
	}.ServeWithHandler(t, h.createUser)

	assert.Equal(t, http.StatusCreated, rec.Code)
	assert.Equal(t, userJSON+"\n", rec.Body.String())
}

func TestGetUser(t *testing.T) {
	// Setup
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	c.SetPath("/users/:email")
	c.SetPathValues(echo.PathValues{
		{Name: "email", Value: "jon@labstack.com"},
	})
	h := &controller{mockDB}

	// Assertions
	if assert.NoError(t, h.getUser(c)) {
		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Equal(t, userJSON, rec.Body.String())
	}
}

func TestGetUserWithEchoTest(t *testing.T) {
	c, rec := echotest.ContextConfig{
		PathValues: echo.PathValues{
			{Name: "email", Value: "jon@labstack.com"},
		},
		Headers: map[string][]string{
			echo.HeaderContentType: {echo.MIMEApplicationJSON},
		},
		JSONBody: []byte(userJSON),
	}.ToContextRecorder(t)

	h := &controller{mockDB}

	// Assertions
	if assert.NoError(t, h.getUser(c)) {
		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Equal(t, userJSON+"\n", rec.Body.String())
	}
}
```

### Using Form Payload

```go
// import "net/url"
f := make(url.Values)
f.Set("name", "Jon Snow")
f.Set("email", "jon@labstack.com")
req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(f.Encode()))
req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationForm)
```

Multipart form payload:
```go
func TestContext_MultipartForm(t *testing.T) {
	testConf := echotest.ContextConfig{
		MultipartForm: &echotest.MultipartForm{
			Fields: map[string]string{
				"key": "value",
			},
			Files: []echotest.MultipartFormFile{
				{
					Fieldname: "file",
					Filename:  "test.json",
					Content:   echotest.LoadBytes(t, "testdata/test.json"),
				},
			},
		},
	}
	c := testConf.ToContext(t)

	assert.Equal(t, "value", c.FormValue("key"))
	assert.Equal(t, http.MethodPost, c.Request().Method)
	assert.Equal(t, true, strings.HasPrefix(c.Request().Header.Get(echo.HeaderContentType), "multipart/form-data; boundary="))

	fv, err := c.FormFile("file")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, "test.json", fv.Filename)
}
```

### Setting Path Params

```go
c.SetPathValues(echo.PathValues{
	{Name: "id", Value: "1"},
	{Name: "email", Value: "jon@labstack.com"},
})
```

### Setting Query Params

```go
// import "net/url"
q := make(url.Values)
q.Set("email", "jon@labstack.com")
req := httptest.NewRequest(http.MethodGet, "/?"+q.Encode(), nil)
```

## Testing Middleware

```go
func TestCreateUserWithEchoTest2(t *testing.T) {
	handler := func(c *echo.Context) error {
		return c.JSON(http.StatusTeapot, fmt.Sprintf("email: %s", c.Param("email")))
	}
	middleware := func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			c.Set("user_id", int64(1234))
			return next(c)
		}
	}

	c, rec := echotest.ContextConfig{
		PathValues: echo.PathValues{{Name: "email", Value: "jon@labstack.com"}},
	}.ToContextRecorder(t)

	err := middleware(handler)(c)
	if err != nil {
		t.Fatal(err)
	}
	// check that middleware set the value
	userID, err := echo.ContextGet[int64](c, "user_id")
	assert.NoError(t, err)
	assert.Equal(t, int64(1234), userID)

	// check that handler returned the correct response
	assert.Equal(t, http.StatusTeapot, rec.Code)
}
```

For now you can look into built-in middleware [test cases](https://github.com/labstack/echo/tree/master/middleware).
