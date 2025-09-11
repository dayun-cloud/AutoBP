export namespace main {
	
	export class Champion {
	    id: number;
	    name: string;
	
	    static createFrom(source: any = {}) {
	        return new Champion(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	    }
	}
	export class int {
	
	
	    static createFrom(source: any = {}) {
	        return new int(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	
	    }
	}
	export class Config {
	    auto_accept_enabled: boolean;
	    preselect_enabled: boolean;
	    auto_ban_enabled: boolean;
	    auto_pick_enabled: boolean;
	    preselect_champion_id?: number;
	    auto_ban_champion_id?: number;
	    auto_pick_champion_id?: number;
	    position_champions: Record<string, number>;
	
	    static createFrom(source: any = {}) {
	        return new Config(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.auto_accept_enabled = source["auto_accept_enabled"];
	        this.preselect_enabled = source["preselect_enabled"];
	        this.auto_ban_enabled = source["auto_ban_enabled"];
	        this.auto_pick_enabled = source["auto_pick_enabled"];
	        this.preselect_champion_id = source["preselect_champion_id"];
	        this.auto_ban_champion_id = source["auto_ban_champion_id"];
	        this.auto_pick_champion_id = source["auto_pick_champion_id"];
	        this.position_champions = source["position_champions"];
	    }
	}
	export class LCUStatus {
	    connected: boolean;
	    client_status: string;
	    champ_select: Record<string, any>;
	
	    static createFrom(source: any = {}) {
	        return new LCUStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.connected = source["connected"];
	        this.client_status = source["client_status"];
	        this.champ_select = source["champ_select"];
	    }
	}
	export class PlayerProfile {
	    summonerName: string;
	    summonerLevel: number;
	    profileIconId: number;
	    accountId: number;
	    summonerId: number;
	    puuid: string;
	
	    static createFrom(source: any = {}) {
	        return new PlayerProfile(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.summonerName = source["summonerName"];
	        this.summonerLevel = source["summonerLevel"];
	        this.profileIconId = source["profileIconId"];
	        this.accountId = source["accountId"];
	        this.summonerId = source["summonerId"];
	        this.puuid = source["puuid"];
	    }
	}
	export class RankedStats {
	    queueType: string;
	    tier: string;
	    rank: string;
	    leaguePoints: number;
	    wins: number;
	    losses: number;
	    hotStreak: boolean;
	    veteran: boolean;
	    freshBlood: boolean;
	    inactive: boolean;
	
	    static createFrom(source: any = {}) {
	        return new RankedStats(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.queueType = source["queueType"];
	        this.tier = source["tier"];
	        this.rank = source["rank"];
	        this.leaguePoints = source["leaguePoints"];
	        this.wins = source["wins"];
	        this.losses = source["losses"];
	        this.hotStreak = source["hotStreak"];
	        this.veteran = source["veteran"];
	        this.freshBlood = source["freshBlood"];
	        this.inactive = source["inactive"];
	    }
	}

}

