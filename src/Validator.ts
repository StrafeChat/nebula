import { NextFunction, Request, Response } from "express";
import { cassandra } from ".";

export class Validator {

    public static async verifyToken(req: Request, res: Response, next: NextFunction) {
        const token = req.headers["authorization"];
        if (!token) return res.status(401).json({ message: "Unauthorized." });

        const parts = token.split(".");
        if (parts.length !== 3) return res.status(401).json({ message: "Unauthorized." });

        try {
            const id = Buffer.from(parts[0], 'base64').toString("utf-8");
            const timestamp = parseInt(Buffer.from(parts[1], 'base64').toString("utf-8"), 10);
            const secret = Buffer.from(parts[2], 'base64').toString("utf-8");

            const user = await cassandra.execute(`
            SELECT id, last_pass_reset, secret, avatar, banner, created_at, edited_at FROM ${cassandra.keyspace}.users
            WHERE id=?
            LIMIT 1;
            `, [id]);

            if (user.rowLength < 1 || user.rows[0].get("last_pass_reset") > timestamp || user.rows[0].get("secret") !== secret) {
                return res.status(401).json({ message: "Unauthorized." });
            }

            (req as any).user = user.rows[0];
            next();
        } catch (error) {
            console.error(error);
            return res.status(500).json({ message: "Internal Server Error." });
        }
    }
}