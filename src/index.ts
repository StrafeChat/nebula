import bodyParser from "body-parser";
import cors from "cors";
import express from "express";
import fs from "fs";
import helmet from "helmet";
import { Logger } from "./helpers/logger";
import { PORT } from "../config";
import database, { redis } from "./database";

const app = express();

app.use(bodyParser.json());
app.use(helmet());
app.use(cors());

app.set('trust proxy', 1);
app.disable('x-powered-by');

app.use((req, res, next) => {
    res.header('Access-Control-Allow-Origin', '*');
    res.header('Access-Control-Allow-Headers', '*');
    res.header('Access-Control-Allow-Methods', '*');

    if (req.method === 'OPTIONS') {
        res.status(200).send();
    } else {
        next();
    }
})

const init = async () => {
    if (!fs.existsSync("static")) fs.mkdir("static", () => { });

    const folders = ["avatars"];

    for (const folder of folders) {
        fs.mkdir(`static/${folder}`, { recursive: true }, () => { });
    }
}

const startServer = async () => {
    const routes = fs.readdirSync("src/routes");

    for (const route of routes) {
        app.use(`/${route.replace(".ts", '')}`, require(`./routes/${route}`).default);
    }

    app.listen(PORT, () => {
        Logger.success(`Nebula is listening on port ${PORT}!`);
    });

    app.use((_req, res) => {
        res.status(404).json({ message: "The resource you are looking for does not exist!" });
    });
}

(async () => {
    Logger.start();
    await init();
    await database.init();
    await startServer();
    await redis.subscribe("equinox", (message, channel) => {
        console.log(message, channel);
    })
})();