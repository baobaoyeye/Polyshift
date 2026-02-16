const { PluginServer } = require('../../sdk/js');

const server = new PluginServer();

server.registerHandler(async (req) => {
  console.error(`[NodeJS Plugin] Received request: ${req.path}`);

  if (req.path === '/api/node') {
    const greeting = server.getConfig('greeting') || 'Hello';
    return {
      status_code: 200,
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ message: `${greeting} from Node.js Plugin!` })
    };
  }

  return {
    status_code: 200,
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ message: `Node.js Echo: ${req.path}` })
  };
});

server.start();
