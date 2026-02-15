export namespace shared {
	
	export class ConfigRequest {
	    mode: string;
	    serverPort: number;
	    serverAddress: string;
	    autoStart: boolean;
	    syncMode: string;
	
	    static createFrom(source: any = {}) {
	        return new ConfigRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.mode = source["mode"];
	        this.serverPort = source["serverPort"];
	        this.serverAddress = source["serverAddress"];
	        this.autoStart = source["autoStart"];
	        this.syncMode = source["syncMode"];
	    }
	}
	export class StatusResponse {
	    running: boolean;
	    client_connected: boolean;
	    client_count: number;
	    last_copied: string;
	    message: string;
	
	    static createFrom(source: any = {}) {
	        return new StatusResponse(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.running = source["running"];
	        this.client_connected = source["client_connected"];
	        this.client_count = source["client_count"];
	        this.last_copied = source["last_copied"];
	        this.message = source["message"];
	    }
	}

}

