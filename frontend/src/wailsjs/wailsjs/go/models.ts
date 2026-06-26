export namespace frontendbridge {
	
	export class SpeechDiagnostics {
	    model: string;
	    task: string;
	    temperature: number;
	    bestOf: number;
	    beamSize: number;
	    channelLanguages: Record<string, string>;
	    lastChannelId: string;
	    lastLanguage: string;
	    lastInferenceMs: number;
	    lastTextChars: number;
	    lastError: string;
	    updatedAt: string;
	
	    static createFrom(source: any = {}) {
	        return new SpeechDiagnostics(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.model = source["model"];
	        this.task = source["task"];
	        this.temperature = source["temperature"];
	        this.bestOf = source["bestOf"];
	        this.beamSize = source["beamSize"];
	        this.channelLanguages = source["channelLanguages"];
	        this.lastChannelId = source["lastChannelId"];
	        this.lastLanguage = source["lastLanguage"];
	        this.lastInferenceMs = source["lastInferenceMs"];
	        this.lastTextChars = source["lastTextChars"];
	        this.lastError = source["lastError"];
	        this.updatedAt = source["updatedAt"];
	    }
	}
	export class OSCSettings {
	    enabled: boolean;
	    destination: string;
	    port: number;
	
	    static createFrom(source: any = {}) {
	        return new OSCSettings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.enabled = source["enabled"];
	        this.destination = source["destination"];
	        this.port = source["port"];
	    }
	}
	export class KeywordRule {
	    phrase: string;
	    highlightColor: string;
	    wholeWord: boolean;
	    triggerEnabled: boolean;
	    oscAddress: string;
	    oscArguments: string[];
	
	    static createFrom(source: any = {}) {
	        return new KeywordRule(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.phrase = source["phrase"];
	        this.highlightColor = source["highlightColor"];
	        this.wholeWord = source["wholeWord"];
	        this.triggerEnabled = source["triggerEnabled"];
	        this.oscAddress = source["oscAddress"];
	        this.oscArguments = source["oscArguments"];
	    }
	}
	export class Partial {
	    channelId: string;
	    channelName: string;
	    color: string;
	    icon: string;
	    timestamp: string;
	    text: string;
	
	    static createFrom(source: any = {}) {
	        return new Partial(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.channelId = source["channelId"];
	        this.channelName = source["channelName"];
	        this.color = source["color"];
	        this.icon = source["icon"];
	        this.timestamp = source["timestamp"];
	        this.text = source["text"];
	    }
	}
	export class Highlight {
	    phrase: string;
	    color: string;
	    start: number;
	    end: number;
	
	    static createFrom(source: any = {}) {
	        return new Highlight(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.phrase = source["phrase"];
	        this.color = source["color"];
	        this.start = source["start"];
	        this.end = source["end"];
	    }
	}
	export class Entry {
	    id: string;
	    channelId: string;
	    channelName: string;
	    color: string;
	    icon: string;
	    timestamp: string;
	    text: string;
	    keywords: string[];
	    highlights: Highlight[];
	    finalized: boolean;
	
	    static createFrom(source: any = {}) {
	        return new Entry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.channelId = source["channelId"];
	        this.channelName = source["channelName"];
	        this.color = source["color"];
	        this.icon = source["icon"];
	        this.timestamp = source["timestamp"];
	        this.text = source["text"];
	        this.keywords = source["keywords"];
	        this.highlights = this.convertValues(source["highlights"], Highlight);
	        this.finalized = source["finalized"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class Snapshot {
	    entries: Entry[];
	    partials: Record<string, Partial>;
	
	    static createFrom(source: any = {}) {
	        return new Snapshot(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.entries = this.convertValues(source["entries"], Entry);
	        this.partials = this.convertValues(source["partials"], Partial, true);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class Device {
	    id: string;
	    name: string;
	
	    static createFrom(source: any = {}) {
	        return new Device(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	    }
	}
	export class Channel {
	    id: string;
	    name: string;
	    color: string;
	    icon: string;
	    inputDevice: string;
	    language: string;
	    gainDb: number;
	    enabled: boolean;
	
	    static createFrom(source: any = {}) {
	        return new Channel(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.color = source["color"];
	        this.icon = source["icon"];
	        this.inputDevice = source["inputDevice"];
	        this.language = source["language"];
	        this.gainDb = source["gainDb"];
	        this.enabled = source["enabled"];
	    }
	}
	export class BootstrapPayload {
	    channels: Channel[];
	    inputDevices: Device[];
	    audioLevels: Record<string, number>;
	    snapshot: Snapshot;
	    keywords: KeywordRule[];
	    osc: OSCSettings;
	    speech: SpeechDiagnostics;
	    status: string;
	    engineLabel: string;
	    keywordCount: number;
	
	    static createFrom(source: any = {}) {
	        return new BootstrapPayload(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.channels = this.convertValues(source["channels"], Channel);
	        this.inputDevices = this.convertValues(source["inputDevices"], Device);
	        this.audioLevels = source["audioLevels"];
	        this.snapshot = this.convertValues(source["snapshot"], Snapshot);
	        this.keywords = this.convertValues(source["keywords"], KeywordRule);
	        this.osc = this.convertValues(source["osc"], OSCSettings);
	        this.speech = this.convertValues(source["speech"], SpeechDiagnostics);
	        this.status = source["status"];
	        this.engineLabel = source["engineLabel"];
	        this.keywordCount = source["keywordCount"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	export class ChannelAddInput {
	    id: string;
	    name: string;
	    color: string;
	    icon: string;
	    inputDevice: string;
	    language: string;
	    gainDb: number;
	    enabled: boolean;
	
	    static createFrom(source: any = {}) {
	        return new ChannelAddInput(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.color = source["color"];
	        this.icon = source["icon"];
	        this.inputDevice = source["inputDevice"];
	        this.language = source["language"];
	        this.gainDb = source["gainDb"];
	        this.enabled = source["enabled"];
	    }
	}
	export class ChannelUpdateInput {
	    id: string;
	    name: string;
	    color: string;
	    icon: string;
	    inputDevice: string;
	    language: string;
	    gainDb: number;
	    enabled: boolean;
	
	    static createFrom(source: any = {}) {
	        return new ChannelUpdateInput(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.color = source["color"];
	        this.icon = source["icon"];
	        this.inputDevice = source["inputDevice"];
	        this.language = source["language"];
	        this.gainDb = source["gainDb"];
	        this.enabled = source["enabled"];
	    }
	}
	
	
	
	
	export class KeywordRuleInput {
	    phrase: string;
	    highlightColor: string;
	    wholeWord: boolean;
	    triggerEnabled: boolean;
	    oscAddress: string;
	    oscArguments: string[];
	
	    static createFrom(source: any = {}) {
	        return new KeywordRuleInput(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.phrase = source["phrase"];
	        this.highlightColor = source["highlightColor"];
	        this.wholeWord = source["wholeWord"];
	        this.triggerEnabled = source["triggerEnabled"];
	        this.oscAddress = source["oscAddress"];
	        this.oscArguments = source["oscArguments"];
	    }
	}
	
	export class OSCSettingsInput {
	    enabled: boolean;
	    destination: string;
	    port: number;
	
	    static createFrom(source: any = {}) {
	        return new OSCSettingsInput(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.enabled = source["enabled"];
	        this.destination = source["destination"];
	        this.port = source["port"];
	    }
	}
	
	

}

