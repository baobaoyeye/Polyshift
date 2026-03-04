const { NodeSDK } = require('@opentelemetry/sdk-node');
const { getNodeAutoInstrumentations } = require('@opentelemetry/auto-instrumentations-node');
const { OTLPTraceExporter } = require('@opentelemetry/exporter-trace-otlp-grpc');
const { resourceFromAttributes } = require('@opentelemetry/resources');
const { SemanticResourceAttributes } = require('@opentelemetry/semantic-conventions');
const { ConsoleSpanExporter, SimpleSpanProcessor } = require('@opentelemetry/sdk-trace-base');
const { diag, DiagConsoleLogger, DiagLogLevel } = require('@opentelemetry/api');

// Initialize OpenTelemetry
function initTracing() {
  const serviceName = process.env.OTEL_SERVICE_NAME || 'unknown-plugin';
  const exporterType = process.env.OTEL_TRACES_EXPORTER || 'otlp';

  if (exporterType === 'none') {
    return;
  }

  // Setup diagnostic logging if requested
  if (process.env.OTEL_DIAG_LEVEL) {
    diag.setLogger(new DiagConsoleLogger(), DiagLogLevel[process.env.OTEL_DIAG_LEVEL] || DiagLogLevel.INFO);
  }

  let traceExporter;
  if (exporterType === 'console') {
    traceExporter = new ConsoleSpanExporter();
  } else {
    // Default OTLP
    traceExporter = new OTLPTraceExporter();
  }

  const sdk = new NodeSDK({
    resource: resourceFromAttributes({
      [SemanticResourceAttributes.SERVICE_NAME]: serviceName,
    }),
    traceExporter: traceExporter,
    instrumentations: [getNodeAutoInstrumentations()],
  });

  try {
    sdk.start();
    console.log(`[NodeJS Plugin] Tracing initialized for ${serviceName} with exporter ${exporterType}`);
  } catch (error) {
    console.error('[NodeJS Plugin] Error initializing tracing', error);
  }

  // Graceful shutdown
  process.on('SIGTERM', () => {
    sdk.shutdown()
      .then(() => console.log('Tracing terminated'))
      .catch((error) => console.log('Error terminating tracing', error))
      .finally(() => process.exit(0));
  });
}

// Initialize tracing before other imports
initTracing();

const grpc = require('@grpc/grpc-js');
const protoLoader = require('@grpc/proto-loader');
const path = require('path');

// Proto file path - assuming we run from project root or sdk/js is in expected location
// We'll try to resolve it relative to this file
const PROTO_PATH = path.resolve(__dirname, '../../proto/plugin/plugin.proto');

const packageDefinition = protoLoader.loadSync(PROTO_PATH, {
  keepCase: true,
  longs: String,
  enums: String,
  defaults: true,
  oneofs: true,
});

const pluginProto = grpc.loadPackageDefinition(packageDefinition).plugin;

class PluginServer {
  constructor() {
    this.server = new grpc.Server();
    this.handler = null;
    this.config = {};
  }

  // User registers their handler function here
  registerHandler(handlerFunc) {
    this.handler = handlerFunc;
  }

  getConfig(key) {
    return this.config[key];
  }

  // gRPC Implementation Methods
  init(call, callback) {
    this.config = call.request.config;
    console.error(`[NodeJS Plugin] Initialized with config: ${JSON.stringify(this.config)}`);
    callback(null, { success: true });
  }

  healthCheck(call, callback) {
    callback(null, { status: "SERVING", message: "OK" });
  }

  shutdown(call, callback) {
    console.error("[NodeJS Plugin] Shutting down...");
    callback(null, {});
    setTimeout(() => {
      this.server.forceShutdown();
      process.exit(0);
    }, 100);
  }

  async handleRequest(call, callback) {
    const req = call.request;
    
    if (!this.handler) {
      return callback(null, {
        status_code: 500,
        error_message: "Handler not registered"
      });
    }

    try {
      const response = await this.handler(req);
      
      // Ensure body is bytes (Buffer in Node.js)
      let body = response.body;
      if (typeof body === 'string') {
        body = Buffer.from(body);
      } else if (!(body instanceof Buffer)) {
        // If it's an object, try to stringify
        body = Buffer.from(JSON.stringify(body));
      }

      callback(null, {
        status_code: response.status_code || 200,
        headers: response.headers || {},
        body: body
      });
    } catch (err) {
      console.error(`[NodeJS Plugin] Handler error: ${err}`);
      callback(null, {
        status_code: 500,
        error_message: err.toString()
      });
    }
  }

  start() {
    // Add service
    this.server.addService(pluginProto.PluginService.service, {
      Init: this.init.bind(this),
      HealthCheck: this.healthCheck.bind(this),
      Shutdown: this.shutdown.bind(this),
      HandleRequest: this.handleRequest.bind(this)
    });

    // Bind to random port
    this.server.bindAsync('127.0.0.1:0', grpc.ServerCredentials.createInsecure(), (err, port) => {
      if (err) {
        console.error(`[NodeJS Plugin] Failed to bind: ${err}`);
        process.exit(1);
      }
      
      // Output address for Core to capture
      // Use console.log for stdout (captured by Core)
      // Use console.error for logging (visible in terminal if Core forwards stderr)
      console.log(`|PLUGIN_ADDR|127.0.0.1:${port}|`);
      console.error(`[NodeJS Plugin] Listening on 127.0.0.1:${port}`);
      
      this.server.start();
    });
  }
}

module.exports = {
  PluginServer
};
