const bigInt = require("snarkjs").bigInt;
const { newMemEmptyTrie, buildPoseidon } = require("circomlibjs");

if (process.argv.length !== 3) {
  throw new Error("Invalid arguments length");
}
const inputsJSON = process.argv[2];
const inputs = JSON.parse(inputsJSON);
const proof = {
  A: ["1", "2", "12"],
  B: ["3", "4", "13", "14", "23", "24"],
  C: ["5", "6", "56"],
  PublicInputs: ["100", "101", "102"]
};
console.log(JSON.stringify(proof));
