# Bearer Token Authentication Examples

All API endpoints support two authentication methods:
- **Cookie-based** — set automatically by browsers after login, no extra work needed
- **Bearer token** — use the `access_token` from the login response in an `Authorization: Bearer <token>` header

This file shows Bearer token usage across different clients.

---

## Login and get token

All examples below start with a login request. The `access_token` from the response is used in subsequent requests.

**Login response:**
```json
{
  "message": "You logged in",
  "access_token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."
}
```

---

## curl

```bash
# Login and capture token
TOKEN=$(curl -s -X POST http://localhost:8080/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"user@example.com","password":"yourpassword"}' \
  | grep -o '"access_token":"[^"]*"' | cut -d'"' -f4)

# List boards
curl -H "Authorization: Bearer $TOKEN" http://localhost:8080/v1/boards

# Get a board
curl -H "Authorization: Bearer $TOKEN" http://localhost:8080/v1/b

# Post a message
curl -X POST http://localhost:8080/v1/b/1 \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"text":"Hello from curl"}'
```

---

## JavaScript (fetch)

```javascript
const BASE = 'http://localhost:8080/v1';

async function login(email, password) {
  const res = await fetch(`${BASE}/auth/login`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ email, password }),
  });
  const data = await res.json();
  return data.access_token;
}

async function apiFetch(token, path, options = {}) {
  return fetch(`${BASE}${path}`, {
    ...options,
    headers: {
      'Authorization': `Bearer ${token}`,
      'Content-Type': 'application/json',
      ...options.headers,
    },
  });
}

// Usage
const token = await login('user@example.com', 'yourpassword');
const res = await apiFetch(token, '/boards');
const boards = await res.json();
```

---

## Python (requests)

```python
import requests

BASE = 'http://localhost:8080/v1'

def login(email, password):
    r = requests.post(f'{BASE}/auth/login', json={'email': email, 'password': password})
    r.raise_for_status()
    return r.json()['access_token']

def make_session(token):
    s = requests.Session()
    s.headers.update({'Authorization': f'Bearer {token}'})
    return s

# Usage
token = login('user@example.com', 'yourpassword')
s = make_session(token)

boards = s.get(f'{BASE}/boards').json()
threads = s.get(f'{BASE}/b').json()
s.post(f'{BASE}/b/1', json={'text': 'Hello from Python'})
```

---

## Node.js

```javascript
const BASE = 'http://localhost:8080/v1';

async function login(email, password) {
  const res = await fetch(`${BASE}/auth/login`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ email, password }),
  });
  const data = await res.json();
  return data.access_token;
}

class ItchanClient {
  constructor(token) {
    this.token = token;
    this.headers = {
      'Authorization': `Bearer ${token}`,
      'Content-Type': 'application/json',
    };
  }

  async getBoards() {
    const res = await fetch(`${BASE}/boards`, { headers: this.headers });
    return res.json();
  }

  async postMessage(board, thread, text) {
    const res = await fetch(`${BASE}/${board}/${thread}`, {
      method: 'POST',
      headers: this.headers,
      body: JSON.stringify({ text }),
    });
    return res.json();
  }
}

// Usage
const token = await login('user@example.com', 'yourpassword');
const client = new ItchanClient(token);
const boards = await client.getBoards();
```

---

## React Native

```javascript
import AsyncStorage from '@react-native-async-storage/async-storage';

const BASE = 'http://localhost:8080/v1';
const TOKEN_KEY = 'itchan_token';

async function login(email, password) {
  const res = await fetch(`${BASE}/auth/login`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ email, password }),
  });
  const data = await res.json();
  await AsyncStorage.setItem(TOKEN_KEY, data.access_token);
  return data.access_token;
}

async function apiFetch(path, options = {}) {
  const token = await AsyncStorage.getItem(TOKEN_KEY);
  return fetch(`${BASE}${path}`, {
    ...options,
    headers: {
      'Authorization': `Bearer ${token}`,
      'Content-Type': 'application/json',
      ...options.headers,
    },
  });
}

async function logout() {
  await fetch(`${BASE}/auth/logout`, {
    method: 'POST',
    headers: { 'Authorization': `Bearer ${await AsyncStorage.getItem(TOKEN_KEY)}` },
  });
  await AsyncStorage.removeItem(TOKEN_KEY);
}

// Usage
await login('user@example.com', 'yourpassword');
const res = await apiFetch('/boards');
const boards = await res.json();
```

---

## Flutter / Dart

```dart
import 'dart:convert';
import 'package:http/http.dart' as http;
import 'package:flutter_secure_storage/flutter_secure_storage.dart';

const base = 'http://localhost:8080/v1';
const storage = FlutterSecureStorage();

Future<String> login(String email, String password) async {
  final res = await http.post(
    Uri.parse('$base/auth/login'),
    headers: {'Content-Type': 'application/json'},
    body: jsonEncode({'email': email, 'password': password}),
  );
  final data = jsonDecode(res.body);
  final token = data['access_token'] as String;
  await storage.write(key: 'itchan_token', value: token);
  return token;
}

Future<http.Response> apiFetch(String path, {String method = 'GET', Object? body}) async {
  final token = await storage.read(key: 'itchan_token');
  final headers = {
    'Authorization': 'Bearer $token',
    'Content-Type': 'application/json',
  };
  final uri = Uri.parse('$base$path');
  return switch (method) {
    'POST' => http.post(uri, headers: headers, body: jsonEncode(body)),
    _ => http.get(uri, headers: headers),
  };
}

// Usage
await login('user@example.com', 'yourpassword');
final res = await apiFetch('/boards');
final boards = jsonDecode(res.body);
```

---

## Go

```go
package main

import (
    "bytes"
    "encoding/json"
    "fmt"
    "net/http"
)

const base = "http://localhost:8080/v1"

type Client struct {
    token      string
    httpClient *http.Client
}

func Login(email, password string) (*Client, error) {
    body, _ := json.Marshal(map[string]string{"email": email, "password": password})
    res, err := http.Post(base+"/auth/login", "application/json", bytes.NewReader(body))
    if err != nil {
        return nil, err
    }
    defer res.Body.Close()

    var data struct {
        AccessToken string `json:"access_token"`
    }
    if err := json.NewDecoder(res.Body).Decode(&data); err != nil {
        return nil, err
    }
    return &Client{token: data.AccessToken, httpClient: &http.Client{}}, nil
}

func (c *Client) Get(path string) (*http.Response, error) {
    req, _ := http.NewRequest("GET", base+path, nil)
    req.Header.Set("Authorization", "Bearer "+c.token)
    return c.httpClient.Do(req)
}

func (c *Client) Post(path string, payload any) (*http.Response, error) {
    body, _ := json.Marshal(payload)
    req, _ := http.NewRequest("POST", base+path, bytes.NewReader(body))
    req.Header.Set("Authorization", "Bearer "+c.token)
    req.Header.Set("Content-Type", "application/json")
    return c.httpClient.Do(req)
}

func main() {
    client, err := Login("user@example.com", "yourpassword")
    if err != nil {
        panic(err)
    }

    res, _ := client.Get("/boards")
    defer res.Body.Close()
    fmt.Println(res.Status)
}
```
