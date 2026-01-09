export namespace config {
	
	export class Config {
	    mode: string;
	    serverPort: number;
	    serverAddress: string;
	    autoStart: boolean;
	
	    static createFrom(source: any = {}) {
	        return new Config(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.mode = source["mode"];
	        this.serverPort = source["serverPort"];
	        this.serverAddress = source["serverAddress"];
	        this.autoStart = source["autoStart"];
	    }
	}

}

