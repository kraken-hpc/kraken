# Websocket Module
The websocket module allows for a client to subscribe to a stream of kraken events. Currently the Module supports subscribing to these events:

- STATE_CHANGE
- STATE_MUTATION
- DISCOVERY

## How to use websocket with Kraken
These steps are assuming that Kraken is running and is built with the websocket module. It also assumes that both the restapi module and websocket module are in RUN state. 

1. Retrieve the websocket information from the restapi by making a GET request to `http://{RESTAPI_IP}:{RESTAPI_PORT}/ws`  
    - This will return an object of this schema:
        ```JSON
        {
          "websocket": {
            "host": "{WEBSOCKET_IP}", 
            "port": "{WEBSOCKET_PORT}",
            "url": "{WEBSOCKET_URL}"
          }
        }
        ```

2. You can now open a websocket with the information provided. Here's an example in javascript with fetch and WebSocket:
    ```javascript
    fetch("http://{RESTAPI_IP}:{RESTAPI_PORT}/ws")
      .then(resp => resp.json())
      .then(json => {
        const wsurl = `ws://${json.websocket.host}:${json.websocket.port}${json.websocket.url}`
        websocket = new WebSocket(wsurl)
    ```

3. Once a websocket connection is established you can send a subscription request to subscribe to an event stream. Here's another javascript example:
    ```javascript
    this.websocket.send(JSON.stringify({ command: 'SUBSCRIBE', type: 'STATE_CHANGE' }))
    this.websocket.send(JSON.stringify({ command: 'SUBSCRIBE', type: 'STATE_MUTATION' }))
    this.websocket.send(JSON.stringify({ command: 'SUBSCRIBE', type: 'DISCOVERY' }))
    ```