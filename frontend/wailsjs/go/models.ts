export namespace catalog {
	
	export class ModCard {
	    id: string;
	    title: string;
	    url: string;
	    imageUrl: string;
	    category: string;
	    author: string;
	    published: string;
	    views: string;
	    downloads: string;
	    size: string;
	    page: number;
	
	    static createFrom(source: any = {}) {
	        return new ModCard(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.title = source["title"];
	        this.url = source["url"];
	        this.imageUrl = source["imageUrl"];
	        this.category = source["category"];
	        this.author = source["author"];
	        this.published = source["published"];
	        this.views = source["views"];
	        this.downloads = source["downloads"];
	        this.size = source["size"];
	        this.page = source["page"];
	    }
	}
	export class ModDetails {
	    id: string;
	    title: string;
	    url: string;
	    imageUrl: string;
	    category: string;
	    author: string;
	    published: string;
	    views: string;
	    downloads: string;
	    size: string;
	    page: number;
	    description: string;
	    screenshots: string[];
	
	    static createFrom(source: any = {}) {
	        return new ModDetails(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.title = source["title"];
	        this.url = source["url"];
	        this.imageUrl = source["imageUrl"];
	        this.category = source["category"];
	        this.author = source["author"];
	        this.published = source["published"];
	        this.views = source["views"];
	        this.downloads = source["downloads"];
	        this.size = source["size"];
	        this.page = source["page"];
	        this.description = source["description"];
	        this.screenshots = source["screenshots"];
	    }
	}
	export class SearchResult {
	    mods: ModCard[];
	    nextPage: number;
	    hasMore: boolean;
	
	    static createFrom(source: any = {}) {
	        return new SearchResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.mods = this.convertValues(source["mods"], ModCard);
	        this.nextPage = source["nextPage"];
	        this.hasMore = source["hasMore"];
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

}

export namespace main {
	
	export class AppSettings {
	    outputPath: string;
	    gamePath: string;
	
	    static createFrom(source: any = {}) {
	        return new AppSettings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.outputPath = source["outputPath"];
	        this.gamePath = source["gamePath"];
	    }
	}
	export class InstalledMod {
	    name: string;
	    path: string;
	    size: number;
	    disabled: boolean;
	
	    static createFrom(source: any = {}) {
	        return new InstalledMod(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.path = source["path"];
	        this.size = source["size"];
	        this.disabled = source["disabled"];
	    }
	}
	export class InstalledModRecord {
	    url: string;
	    title: string;
	    thumbnailUrl: string;
	    path: string;
	    // Go type: time
	    installedAt: any;
	
	    static createFrom(source: any = {}) {
	        return new InstalledModRecord(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.url = source["url"];
	        this.title = source["title"];
	        this.thumbnailUrl = source["thumbnailUrl"];
	        this.path = source["path"];
	        this.installedAt = this.convertValues(source["installedAt"], null);
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
	export class QueueMod {
	    url: string;
	    title: string;
	    thumbnailUrl: string;
	
	    static createFrom(source: any = {}) {
	        return new QueueMod(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.url = source["url"];
	        this.title = source["title"];
	        this.thumbnailUrl = source["thumbnailUrl"];
	    }
	}

}

