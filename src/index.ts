require("dotenv").config();
import { Client } from "cassandra-driver";
import { createClient } from 'redis';
import bodyParser from "body-parser";
import cors from "cors";
import express from "express";
import fs from "fs";
import path from "path";
import { Validator } from "./Validator";

const app = express();

const cassandra = new Client({
    contactPoints: [process.env.CASSANDRA_CONTACT_POINT!],
    localDataCenter: process.env.CASSANDRA_DATA_CENTER,
    keyspace: process.env.CASSANDRA_KEYSPACE
})

const redis = createClient();

app.use(bodyParser.json());
app.use(cors());

app.get("/avatars/:identifier.:ext", (req, res) => {
    res.setHeader("Content-Type", "image/png");
    res.sendFile(`${req.params.identifier}.${req.params.ext}`, { root: path.join(__dirname, "../static/avatars") });
});

app.post("/avatars/", Validator.verifyToken, (req, res) => {
    if (!req.body.data) return res.status(401).json({ message: "Data is required!" });
    const path = `static/avatars/${req.body.user.avatar}.png`;
    fs.writeFileSync(path, Buffer.from(req.body.data, "base64"));

    fs.readFile(path, (err, data) => {
        if (err) {
          console.error(err);
          res.status(500).send("Internal Server Error");
        } else {
          res.writeHead(201, { 'Content-Type': 'image/png' });
          res.end(data, 'binary');
        }
      });
});

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

export { cassandra, redis };