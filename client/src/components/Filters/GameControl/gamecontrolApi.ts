export interface GameHost {
    ip: string;
    host: string;
    blocked: boolean;
    internet_blocked: boolean;
}

export interface GameLab {
    id: string;
    name: string;
    range_start: string;
    range_end: string;
    hosts: GameHost[];
}

export interface GameControlStatus {
    enabled: boolean;
    upstream_url: string;
    restricted: boolean;
    labs: GameLab[];
}

export interface Teacher {
    login: string;
    lab_id: string;
}

export interface PostResult {
    ok: boolean;
    error?: string;
}

export const fetchGameControlStatus = async (): Promise<GameControlStatus | null> => {
    try {
        const res = await fetch('/control/gamecontrol/status');
        if (res.ok) {
            return await res.json();
        }
    } catch (err) {
        console.error('Error fetching GameControl status:', err);
    }

    return null;
};

export const gameControlPost = async (path: string, body: object): Promise<PostResult> => {
    try {
        const res = await fetch(`/control/gamecontrol/${path}`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(body),
        });
        if (res.ok) {
            return { ok: true };
        }

        return { ok: false, error: (await res.text()).trim() || `Error ${res.status}` };
    } catch (err) {
        return { ok: false, error: String(err) };
    }
};
