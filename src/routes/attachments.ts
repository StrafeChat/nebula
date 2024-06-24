import { verifyToken } from "../helpers/validator";
import express from 'express';
import path from 'path';
import crypto from "crypto";
import fs from 'fs';
import sharp from 'sharp';
import bodyParser from "body-parser";

const router = express.Router();

router.use(bodyParser.urlencoded({limit: "25mb", extended: true, parameterLimit: 25000}));

router.get<{ hash: string, file_name: string }>("/:hash/:file_name", (req, res) => {
    const filePath = path.join(__dirname, `../../static/attachments/${req.params.hash}/${req.params.file_name}`);
    if (!req.params.hash || !req.params.file_name) return res.status(404).json({ message: "The attachment you are looking for does not exist" });

    fs.access(filePath, fs.constants.F_OK, (err) => {
        if (err) {
            res.status(404).json({ message: "The attachment you are looking for does not exist." });
        } else {
            res.sendFile(filePath);
        }
    });
});

router.post('/', verifyToken, async (req, res) => {
    if (typeof req.body.file !== "string") {
        return res.status(400).json({ message: "Attachment data must be a base64 string." });
    }

    const { name, type } = req.body;

    if (!name || !type) return res.status(400).json({ message: "You must provide the name of the file."})

    try {
        const base64 = req.body.file.split(',').pop();
        if (!base64) {
            return res.status(400).json({ message: "The data must be a valid base64 data url." });
        }

        const buffer = Buffer.from(base64, 'base64');

        // Check if the file size exceeds 25 MB
        const maxSize = 25 * 1024 * 1024;
        if (buffer.length > maxSize) {
            return res.status(413).json({ message: "File size exceeds the 25 MB limit." });
        }

        const hash = crypto.createHash("sha256");
        hash.update(buffer);
        const hashedAttachment = `${hash.digest("hex")}`;

        const attachmentDir = path.join("static", "attachments", hashedAttachment);

        if (!fs.existsSync(attachmentDir)) {
            fs.mkdirSync(attachmentDir, { recursive: true });
        }

        const filePath = path.join(attachmentDir, name);
        fs.writeFileSync(filePath, buffer);
       
        let width, height;
  
        if (type.startsWith('image/')) {
          const metadata = await sharp(buffer).metadata();
          width = metadata.width;
          height = metadata.height;
        }

        res.status(201).json({
            url: `${process.env.URL}/attachments/${hashedAttachment}/${name}`,
            type,
            metadata: {
                height,
                width
            }
        });
    } catch (err) {
        console.error("ATTACHMENT POST:", err);
        res.status(500).json({ message: "An Internal Server Error has occurred." });
    }
});

export default router;