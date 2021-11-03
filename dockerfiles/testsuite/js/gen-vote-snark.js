const axios = require('axios');
const util = require("util");
const crypto = require('crypto')

const fs = require("fs");
const wc  = require("./witness_calculator.js");
const snarkjs = require("snarkjs");
const ffutils = require("ffjavascript").utils;

async function generateProof(inputs, path) {
  // witness
  let wasmBuff = fs.readFileSync(`${path}/circuit.wasm`);
  let witnessCalculator = await wc(wasmBuff)
  let wtnsBuff = await witnessCalculator.calculateWTNSBin(inputs, 0);
  await fs.writeFile(`${path}/out.wtns`, wtnsBuff, function(err) {
    if (err) console.error(err);
  });

  // proof
  let { proof, publicSignals } = await snarkjs.groth16.prove(`${path}/circuit_final.zkey`, `${path}/out.wtns`)

  // print proof
  console.log(JSON.stringify(proof));

  // check verification of the proof
  let vk = fs.readFileSync(`${path}/verification_key.json`);
  let verified = await snarkjs.groth16.verify(JSON.parse(vk), publicSignals, proof);
  if (!verified) {
    console.error("zk proof failed on verify");
  }
  globalThis.curve_bn128.terminate();
}

async function downloadFile(id, url, path, filename) {
  // console.log("download file:", filename);
  const file = fs.createWriteStream(`${path}/${id}/${filename}`);

  const res = await axios.get(`${url}/${path}/${filename}`, {responseType: "stream"});
  await new Promise(resolve => {
    res.data.pipe(file);
    // console.log("download done:", filename);
    file.on("close", resolve);
  });
}

function randomID(size = 32) {
  return crypto
    .randomBytes(size)
    .toString('hex');
}

if (process.argv.length<3) {
  console.error("ERROR: missing arguments");
  console.log("- usage: > node gen-vote-snark.js inputs");
  console.log("    - inputs: stringified json of the inputs with circuit configuration\n");
  console.log(`- example: > node gen-vote-snark.js '{"circuitIndex":0,"circuitConfig":{"url":"https://raw.githubusercontent.com/vocdoni/zk-circuits-artifacts/master","circuitPath":"zkcensusproof/dev/16","parameters":[16],"localDir":"./circuits","witnessHash":"0CHULXnU4QuUpXheHBhU3bgNCHy1ita7KaqLjVQdQg0=","zKeyHash":"fQmogOFOCBQ7tmpvKOE7Jwevq8eWk84WE/aAg/1wrDE=","vKHash":"9IdqpVDjPeHR9VLcOPqJ9uh+VT/QUXnmk/gvZhzQxqA="},"inputs":{"censusRoot":"2572811951483070062612427687389837770591382077944783952226798455257163673267","censusSiblings":["15440721508236792339907967079473897243963343769856894272383221599521476833315","16470248667904714096542997121090407999951943425763326623244523724751962040658","5123481322991055259806753435672140893837909838310233965972307824376730204553","20048568552934578679583167380914242415857600169523061733410010848771668329354","0"],"index":"0","secretKey":"1135373608990631616873167310934266580394353119526609190224281967520437375495","voteHash":["86019129038062350826752664265838140266","46591815804546548324869865287746686854"],"processId":["186644384700023064769252980238578513178","325091221742051276271295553327167922632"],"nullifier":"20253119235673341127971942087405640320642849225465575273302134459150788088292"}}'`);
  return;
}

const data = JSON.parse(process.argv[2]);
const url = data.circuitConfig.url;
const path = data.circuitConfig.circuitPath;
const inputs = data.inputs;
const id = randomID();

fs.mkdir(`${path}/${id}`, {recursive:true}, async function() {
  await downloadFile(id, url, path, "circuit.wasm");
  await downloadFile(id, url, path, "circuit_final.zkey");
  await downloadFile(id, url, path, "verification_key.json");

  await generateProof(inputs, `${path}/${id}`);
});

