require("dotenv").config();
import { Client } from "cassandra-driver";
import { createClient } from 'redis';
import bodyParser from "body-parser";
import cors from "cors";
import express from "express";
import fs from "fs";
import path from "path";
import { Validator } from "./Validator";
import { createCanvas, loadImage } from "@napi-rs/canvas";

const app = express();

const cassandra = new Client({
    contactPoints: [process.env.CASSANDRA_CONTACT_POINT!],
    localDataCenter: process.env.CASSANDRA_DATA_CENTER,
    keyspace: process.env.CASSANDRA_KEYSPACE
})

const redis = createClient();

app.use(bodyParser.json());
app.use(cors());

try {
    (async () => {
        await redis.connect();
        await cassandra.connect();
        app.listen(process.env.PORT ?? 80, () => {
            console.log("Listening on port " + process.env.PORT);
            init();
        });
    })();
} catch (err) {
    console.log(err);
}

app.get("/avatars/:identifier.:ext", (req, res) => {
    res.setHeader("Content-Type", "image/png");
    res.sendFile(`${req.params.identifier}.${req.params.ext}`, { root: path.join(__dirname, "../static/avatars") });
});

app.post("/avatars/", Validator.verifyToken, async (req, res) => {
    if (!req.body.data) return res.status(401).json({ message: "Data is required!" });

    const path = `static/avatars/${req.body.user.avatar}.png`;
    fs.writeFileSync(path, Buffer.from(req.body.data, "base64"));

    const image = await loadImage(path);
    const canvas = createCanvas(image.width, image.height);
    const ctx = canvas.getContext('2d');
    ctx.drawImage(image, 0, 0, image.width, image.height);
    const imageData = ctx.getImageData(0, 0, canvas.width, canvas.height).data;
    const averageColor = calculateAverageColor(imageData);
    const hexColor = rgbToHex(averageColor[0], averageColor[1], averageColor[2]);

    console.log(req.body.user);

    await cassandra.execute(`
    UPDATE ${cassandra.keyspace}.users
    SET accent_color=?, edited_at=?
    WHERE id=? AND created_at=?
    `, [hexColor, Date.now(), req.body.user.id, req.body.user.created_at], { prepare: true });

    fs.readFile(path, async (err, data) => {
        if (err) {
            console.error(err);
            res.status(500).send("Internal Server Error");
        } else {
            res.writeHead(201, { 'Content-Type': 'image/png' });
            res.end(data, 'binary');
        }
    });
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

function calculateAverageColor(imageData: Uint8ClampedArray) {
    const totalPixels = imageData.length / 4; // Each pixel has 4 values (R, G, B, A)
    let totalRed = 0;
    let totalGreen = 0;
    let totalBlue = 0;

    for (let i = 0; i < imageData.length; i += 4) {
        totalRed += imageData[i];
        totalGreen += imageData[i + 1];
        totalBlue += imageData[i + 2];
    }

    const averageRed = Math.round(totalRed / totalPixels);
    const averageGreen = Math.round(totalGreen / totalPixels);
    const averageBlue = Math.round(totalBlue / totalPixels);

    return [averageRed, averageGreen, averageBlue];
}

function rgbToHex(red: number, green: number, blue: number) {
    const hexRed = red.toString(16).padStart(2, '0');
    const hexGreen = green.toString(16).padStart(2, '0');
    const hexBlue = blue.toString(16).padStart(2, '0');

    return parseInt(`${hexRed}${hexGreen}${hexBlue}`, 16);
}

export { cassandra, redis };