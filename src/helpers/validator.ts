import { NextFunction, Request, Response } from "express";
import { cassandra } from "../database";
import { User } from "../types";

export const verifyToken = async (req: Request, res: Response & { locals: { user: User } }, next: NextFunction) => {
    const token = req.headers["authorization"];
    if (typeof token != "string") return res.status(401).json({ message: "Unauthorized" });
    const splitToken = token.split('.');
    if (splitToken.length < 3) return res.status(401).json({ message: "Unauthorized" });

    const id = atob(splitToken[0]);
    const last_pass_reset = atob(splitToken[1]);
    const secret = atob(splitToken[2]);

    const users = await cassandra.execute(`
    SELECT last_pass_reset, secret, created_at FROM ${cassandra.keyspace}.users
    WHERE id=?
    LIMIT 1;
    `, [id]);

    if (users.rowLength < 1) return res.status(401).json({ message: "Unauthorized" });
    if (!users.rows[0].get("verified") || users.rows[0].get("last_pass_reset") != parseInt(last_pass_reset) || users.rows[0].get("secret") != secret) return res.status(401).json({ message: "Unauthorized" });
    res.locals.user = { ...users.rows[0], id } as unknown as User;
    next();
}