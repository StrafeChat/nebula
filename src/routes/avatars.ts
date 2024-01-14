import crypto from "crypto";
import { Router } from "express";
import fs from "fs";
import path from "path";
import sharp from 'sharp';
import { avatarCount } from "..";
import { cassandra } from "../database";
import { verifyToken } from "../helpers/validator";
import { User } from "../types";
const router = Router();

router.get<{ userId: string, hash: string, ext: string }>("/:userId/:hash.:ext", (req, res) => {
    const filePath = path.join(__dirname, `../../static/avatars/${req.params.userId}/${req.params.hash}.${req.params.ext}`);
    if (!req.params.ext || !req.params.hash || !req.params.userId) return res.status(404).json({ message: "The avatar you are looking for does not exist" });

    fs.access(filePath, fs.constants.F_OK, (err) => {
        if (err) {
            res.status(404).json({ message: "The avatar you are looking for does not exist." });
        } else {
            res.sendFile(filePath);
        }
    });
});

router.post<string, {}, {}, {}, {}, { user: User }>('/', verifyToken, async (_req, res) => {
    if (res.locals.user.avatar) return res.status(409).json({ message: "You already have an avatar. Did you mean to update it instead?" });

    try {
        const index = crypto.randomInt(1, avatarCount);
        const buf = fs.readFileSync(`static/avatars/default/avatar${index}.webp`);
        const hash = crypto.createHash("sha256");
        hash.update(buf);
        const hashedAvatar = `${hash.digest("hex")}`;

        const avatarDir = path.join('static', 'avatars', res.locals.user.id);

        if (!fs.existsSync(avatarDir)) fs.mkdirSync(avatarDir);
        fs.writeFileSync(path.join(avatarDir, `${hashedAvatar}.webp`), buf);

        await cassandra.execute(`
        UPDATE ${cassandra.keyspace}.users
        SET avatar=?
        WHERE id=? AND created_at=?
        `, [`${hashedAvatar}.webp`, res.locals.user.id, res.locals.user.created_at]);

        res.status(201).json({ avatar: `${hashedAvatar}.webp` });
    } catch (err) {
        console.error("AVATAR POST:", err);
        res.status(500).json({ message: "An Internal Server Error has occured." });
    }
});

router.patch<string, {}, {}, { data: string }, {}, { user: User }>('/', verifyToken, async (req, res) => {
    if (typeof req.body.data != "string") return res.status(400).json({ message: "Avatar data must be a base64 string." });

    const regex = /^data:[a-z0-9]+\/[a-z0-9]+;base64,[a-zA-Z0-9+/=]+$/;
    if (!regex.test(req.body.data)) return res.status(400).json({ message: "The data must be a valid base64 data url." });

    if (!res.locals.user.avatar) return res.status(409).json({ message: "You do not have an avatar setup. Did you mean to create one instead?" });

    try {
        const base64 = req.body.data.split(',').pop();
        if (!base64) return res.status(400).json({ message: "The data must be a valid base64 data url." });
        const buf = Buffer.from(base64, "base64");

        let extension = '.webp';

        try {
            const metadata = await sharp(buf).metadata();

            if (metadata.pages! > 1) {
                extension = '.gif';
            }
        } catch (err) {
            console.error("Error determining image type:", err);
            return res.status(500).json({ message: "An error occurred while processing the image." });
        }

        const hash = crypto.createHash("sha256");
        hash.update(buf);
        const hashedAvatar = `${hash.digest("hex")}${extension}`;

        const avatarDir = path.join("static", "avatars", res.locals.user.id);

        if (!fs.existsSync(avatarDir)) fs.mkdirSync(avatarDir);
        await sharp(buf).toFile(path.join(avatarDir, `${hashedAvatar}${extension}`));

        await cassandra.execute(`
        UPDATE ${cassandra.keyspace}.users
        SET avatar=?
        WHERE id=? AND created_at=?
        `, [hashedAvatar, res.locals.user.id, res.locals.user.created_at]);

        res.status(201).json({ avatar: hashedAvatar });
    } catch (err) {
        console.error("AVATAR PATCH:", err);
        res.status(500).json({ message: "An Internal Server Error has occured." });
    }
});

// router.patch<string, {}, {}, { data: string }, {}, { user: User }>("/", verifyToken, (req, res) => {
//     if (typeof req.body.data != "string") return res.status(400).json({ message: "Avatar data must be a base64 string." });

//     const regex = /^data:[a-z0-9]+\/[a-z0-9]+;base64,[a-zA-Z0-9+/=]+$/;
//     if (!regex.test(req.body.data)) return res.status(400).json({ message: "The data must be a valid base64 data url." });


// });

export default router;