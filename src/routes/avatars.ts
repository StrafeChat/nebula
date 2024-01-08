import { Router } from "express";
import path from "path";
import { verifyToken } from "../helpers/validator";
const router = Router();
import fs from "fs";
import { User } from "../types";

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
})

router.patch<string, {}, {}, { data: string }, {}, { user: User }>("/", verifyToken, (req, res) => {
    if (typeof req.body.data != "string") return res.status(400).json({ message: "Avatar data must be a base64 string." });

    const regex = /^data:[a-z0-9]+\/[a-z0-9]+;base64,[a-zA-Z0-9+/=]+$/;
    if (!regex.test(req.body.data)) return res.status(400).json({ message: "The data must be a valid base64 data url." });

    
});

export default router;