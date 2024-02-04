export interface User {
    created_space_ids: string[];
    last_pass_reset: string;
    secret: string;
    avatar?: string;
    id: string;
    created_at: number;
}