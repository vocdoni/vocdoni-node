include "../../node_modules/circomlib/circuits/comparators.circom";
include "../../node_modules/circomlib/circuits/poseidon.circom";
include "../../node_modules/circomlib/circuits/smt/smtverifier.circom";

template SMTVerifierTest(nLevels) {
	signal input key;
	signal input value;
	signal input fnc;
	signal private input oldKey;
	signal private input oldValue;
	signal private input isOld0;
	signal private input siblings[nLevels];
	signal input root;

	component smtV = SMTVerifier(nLevels);
	smtV.enabled <== 1;
	smtV.fnc <== fnc;
	smtV.root <== root;
	for (var i=0; i<nLevels; i++) {
		smtV.siblings[i] <== siblings[i];
	}
	smtV.oldKey <== oldKey;
	smtV.oldValue <== oldValue;
	smtV.isOld0 <== isOld0;
	smtV.key <== key;
	smtV.value <== value;
}
