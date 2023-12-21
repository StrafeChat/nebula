require("dotenv").config();
import { Client } from "cassandra-driver";
import bodyParser from "body-parser";
import cors from "cors";
import express from "express";
import fs from "fs";
import path from "path";
import { Validator } from "./Validator";
import { GIF, decode } from "imagescript";
import crypto from "crypto";

const app = express();

const cassandra = new Client({
    contactPoints: [process.env.SCYLLA_CONTACT_POINT1!, process.env.SCYLLA_CONTACT_POINT2!, process.env.SCYLLA_CONTACT_POINT3!],
    localDataCenter: process.env.SCYLLA_DATA_CENTER,
    credentials: { username: process.env.SCYLLA_USERNAME!, password: process.env.SCYLLA_PASSWORD! },
    keyspace: process.env.SCYLLA_KEYSPACE
})

//const redis = createClient();

app.use(bodyParser.json({ limit: "25mb" }));
app.use(cors());

try {
    (async () => {
        //await redis.connect();
        await cassandra.connect();
        app.listen(process.env.PORT ?? 80, () => {
            console.log("Listening on port " + process.env.PORT);
            init();
        });
    })();
} catch (err) {
    console.log(err);
}

app.get("/", (req, res) => {
    res.send("Hello World!");
});

app.get("/avatars/:userId/:hash.:ext", (req, res) => {
    res.setHeader("Content-Type", "image/png");
    res.sendFile(`${req.params.hash}.${req.params.ext}`, { root: path.join(__dirname, `../static/avatars/${req.params.userId}`) });
});

app.patch("/avatars/", Validator.verifyToken, async (req, res) => {
    try {
        if (!req.body.data) return;
        if (!fs.existsSync(path.join("static", "avatars", req.user!.id))) fs.mkdirSync(path.join("static", "avatars", req.user!.id));

        const buffer = Buffer.from(req.body.data, "base64");
        const hash = crypto.createHash("sha256");
        hash.update(buffer);
        const hashedAvatar = hash.digest("hex");

        const avatarPath = path.join('static', 'avatars', req.user!.id, hashedAvatar);

        let image = await decode(buffer, true);

        if (image instanceof GIF) {
            const accentInt = image[0].averageColor();

            const red = (accentInt >> 24) & 0xFF;
            const green = (accentInt >> 16) & 0xFF;
            const blue = (accentInt >> 8) & 0xFF;

            const accentColor = (red << 16) | (green << 8) | blue;

            fs.writeFileSync(`${avatarPath}.gif`, Buffer.from(req.body.data, "base64"));

            await cassandra.execute(`
            UPDATE ${cassandra.keyspace}.users
            SET accent_color=?, avatar=?, edited_at=?
            WHERE id=? AND created_at=?
            `, [accentColor, `${hashedAvatar}_gif`, Date.now(), req.user!.id, req.user!.created_at], { prepare: true });

            res.status(200).json({ hash: `${hashedAvatar}_gif` });
        } else {
            const accentInt = image.averageColor();

            const red = (accentInt >> 24) & 0xFF;
            const green = (accentInt >> 16) & 0xFF;
            const blue = (accentInt >> 8) & 0xFF;

            const accentColor = (red << 16) | (green << 8) | blue;

            // TODO: Delete old avatars possibly.

            fs.writeFileSync(`${avatarPath}.png`, Buffer.from(req.body.data, "base64"));

            await cassandra.execute(`
            UPDATE ${cassandra.keyspace}.users
            SET accent_color=?, avatar=?, edited_at=?
            WHERE id=? AND created_at=?
            `, [accentColor, `${hashedAvatar}_png`, Date.now(), req.user!.id, req.user!.created_at], { prepare: true });

            res.status(200).json({ hash: `${hashedAvatar}_png` });
        }
    } catch (err) {
        console.log(err);
    }
});

const init = async () => {
    if (!fs.existsSync("static")) {
        fs.mkdir("static", (err) => {
            throw err;
        })
        init();
    };

    const folders = ["avatars", "banners"];

    for (const folder of folders) {
        fs.mkdir(`static/${folder}`, { recursive: true }, (err) => { });
    }
}

export { cassandra };