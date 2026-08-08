export namespace main {
	
	export class FileEntry {
	    relPath: string;
	    displayName: string;
	    source: string;
	    identical: boolean;
	
	    static createFrom(source: any = {}) {
	        return new FileEntry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.relPath = source["relPath"];
	        this.displayName = source["displayName"];
	        this.source = source["source"];
	        this.identical = source["identical"];
	    }
	}
	export class ComparisonResult {
	    leftLabel: string;
	    rightLabel: string;
	    gitMode: boolean;
	    files: FileEntry[];
	
	    static createFrom(source: any = {}) {
	        return new ComparisonResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.leftLabel = source["leftLabel"];
	        this.rightLabel = source["rightLabel"];
	        this.gitMode = source["gitMode"];
	        this.files = this.convertValues(source["files"], FileEntry);
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
	export class DiffLineDTO {
	    type: string;
	    content: string;
	    leftLineNum: number;
	    rightLineNum: number;
	
	    static createFrom(source: any = {}) {
	        return new DiffLineDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.type = source["type"];
	        this.content = source["content"];
	        this.leftLineNum = source["leftLineNum"];
	        this.rightLineNum = source["rightLineNum"];
	    }
	}
	export class SideBySideRowDTO {
	    type: string;
	    leftContent: string;
	    rightContent: string;
	    leftLineNum: number;
	    rightLineNum: number;
	
	    static createFrom(source: any = {}) {
	        return new SideBySideRowDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.type = source["type"];
	        this.leftContent = source["leftContent"];
	        this.rightContent = source["rightContent"];
	        this.leftLineNum = source["leftLineNum"];
	        this.rightLineNum = source["rightLineNum"];
	    }
	}
	export class DiffResult {
	    relPath: string;
	    displayName: string;
	    source: string;
	    leftLabel: string;
	    rightLabel: string;
	    lines: DiffLineDTO[];
	    sideBySide: SideBySideRowDTO[];
	    equal: number;
	    inserted: number;
	    deleted: number;
	    canMerge: boolean;
	
	    static createFrom(source: any = {}) {
	        return new DiffResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.relPath = source["relPath"];
	        this.displayName = source["displayName"];
	        this.source = source["source"];
	        this.leftLabel = source["leftLabel"];
	        this.rightLabel = source["rightLabel"];
	        this.lines = this.convertValues(source["lines"], DiffLineDTO);
	        this.sideBySide = this.convertValues(source["sideBySide"], SideBySideRowDTO);
	        this.equal = source["equal"];
	        this.inserted = source["inserted"];
	        this.deleted = source["deleted"];
	        this.canMerge = source["canMerge"];
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
	
	export class MergeSaveRequest {
	    relPath: string;
	    target: string;
	    selectedInsertions: number[];
	    selectedDeletions: number[];
	
	    static createFrom(source: any = {}) {
	        return new MergeSaveRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.relPath = source["relPath"];
	        this.target = source["target"];
	        this.selectedInsertions = source["selectedInsertions"];
	        this.selectedDeletions = source["selectedDeletions"];
	    }
	}

}

