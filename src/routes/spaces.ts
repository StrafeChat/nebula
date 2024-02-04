import crypto from "crypto";
import { Router } from "express";
import fs from "fs";
import path from "path";
import sharp from "sharp";
import { cassandra } from "../database";
import { verifyToken } from "../helpers/validator";

const router = Router();

router.put('/', verifyToken, async (req, res) => {
    if (typeof req.body.data != "string") return res.status(400).json({ message: "Icon data must be a base64 string." });
    const regex = /^data:[a-z0-9]+\/[a-z0-9]+;base64,[a-zA-Z0-9+/=]+$/;
    if (!regex.test(req.body.data)) return res.status(400).json({ message: "Icon data must be a valid base64 data url." });

    try {
        const base64 = req.body.data.split(',').pop();
        if (!base64) return res.status(400).json({ message: "Icon data must be a valid base64 data url." });

        const query = await cassandra.execute(`
        SELECT owner_id from spaces WHERE id=?
        `, [req.body.space_id])

        if (query.rows.length == 0) return res.status(404).json({ message: "Space not found." });
        if (query.rows[0].get("owner_id") != res.locals.user.id) return res.status(403).json({ message: "You do not have permission to edit this space." });

        const buf = Buffer.from(base64, "base64");
        let extension = '.webp';

        try {
            const metadata = await sharp(buf).metadata();
            if (metadata.pages! > 1) extension = '.gif';
        } catch (err) {
            console.error("Error determining image type:", err);
            return res.status(500).json({ message: "An error occurred while processing the image." });
        }

        const hash = crypto.createHash("sha256");
        hash.update(buf);
        const hashedIcon = `${hash.digest("hex")}${extension}`;
        const iconPath = path.join("static", "spaces", req.body.space_id);

        if (!fs.existsSync(iconPath)) fs.mkdirSync(iconPath);

        await sharp(buf).toFile(path.join(iconPath, `${hashedIcon}${extension}`));

        await cassandra.execute(`
        UPDATE spaces SET icon=? WHERE id=?
        `, [hashedIcon, req.body.space_id]);

        res.status(201).json({ icon: hashedIcon });
    } catch (err) {
        console.error("SPACE ICON PUT:", err);
        res.status(500).json({ message: "An Internal Server Error has occured." });
    }
});

export default router;