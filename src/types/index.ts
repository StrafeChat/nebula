export interface User {
    last_pass_reset: string;
    secret: string;
    avatar?: string;
    id: string;
    created_at: number;
}