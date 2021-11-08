const axios = require('axios');
const util = require("util");

const fs = require("fs");
const wc  = require("./witness_calculator.js");
const snarkjs = require("snarkjs");
const ffutils = require("ffjavascript").utils;

async function generateProof(inputs, path) {
  // witness
  let wasmBuff = fs.readFileSync(path+"circuit.wasm");
  let witnessCalculator = await wc(wasmBuff)
  let wtnsBuff = await witnessCalculator.calculateWTNSBin(inputs,0);
  await fs.writeFile(path+"out.wasm", wtnsBuff, function(err) {
    if (err) console.error(err);
  });

  // proof
  let { proof, publicSignals } = await snarkjs.groth16.prove(path+"circuit_final.zkey", path+"out.wasm")

  // print proof
  console.log(JSON.stringify(proof));

  // check verification of the proof
  let vk = fs.readFileSync(path+"verification_key.json");
  let verified = await snarkjs.groth16.verify(JSON.parse(vk), publicSignals, proof);
  if (!verified) {
    console.error("zk proof failed on verify");
  }
  globalThis.curve_bn128.terminate();
}

async function downloadFile(url, path, filename) {
  // console.log("download file:", filename);
  const file = fs.createWriteStream(path+filename);

  const res = await axios.get(url+path+filename, {responseType: "stream"});
  await new Promise(resolve => {
    res.data.pipe(file);
    // console.log("download done:", filename);
    file.on("close", resolve);
  });
}



if (process.argv.length<5) {
  console.error("ERROR: missing arguments");
  console.log("- usage: > node gen-vote-snark.js url path inputs");
  console.log("    - url: url where the circuits-artifacts is hosted");
  console.log("    - path: path of the circuit to be downloaded");
  console.log("    - inputs: stringified json of the inputs\n");
  console.log(`- example: > node gen-vote-snark.js https://raw.githubusercontent.com/vocdoni/zk-circuits-artifacts/master/ zkcensusproof/dev/16/ '{"censusRoot":"13256983273841966279055596043431919350426357891097196583481278449962353221936","censusSiblings":["0","0","0","0"],"index":"0","secretKey":"3876493977147089964395646989418653640709890493868463039177063670701706079087","voteHash":["302689215824177652345211539748426020171","205062086841587857568430695525160476881"],"processId":["242108076058607163538102198631955675649","142667662805314151155817304537028292174"],"nullifier":"9464482416872469446849476841635888299250846240473623349595304484954843565595"}'`);
  return;
}

let url = process.argv[2];
let path = process.argv[3];
let inputs = JSON.parse(process.argv[4]);

fs.mkdir(path, {recursive:true}, async function() {
  await downloadFile(url, path, "circuit.wasm");
  await downloadFile(url, path, "circuit_final.zkey");
  await downloadFile(url, path, "verification_key.json");

  await generateProof(inputs, path);
});

