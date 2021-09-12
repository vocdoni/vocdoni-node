package vochain

import (
	"math/big"
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/vocdoni/arbo"
	snarkParsers "github.com/vocdoni/go-snark/parsers"
	models "go.vocdoni.io/proto/build/go/models"
)

func TestVoteEnvelopeCheckCaseZkSNARK(t *testing.T) {
	app, err := NewBaseApplication(t.TempDir())
	qt.Assert(t, err, qt.IsNil)

	vkJSON := `
	{
	 "protocol": "groth16",
	 "curve": "bn128",
	 "nPublic": 5,
	 "vk_alpha_1": [
	  "306173029903602630729311012982340700491234800927877597206759293990284528210",
	  "20106879623667538638269962569230954942944631331052974849151810316932051942222",
	  "1"
	 ],
	 "vk_beta_2": [
	  [
	   "6388001092721447861165885641715123572539913010792888888192734071510557971913",
	   "20848050992563430829274627211469581012739038121766063115794423352413732464002"
	  ],
	  [
	   "12394370189260130796904435534938825578562104521748712723133084156874623405320",
	   "19568896438322287296581438238708355738182103315047232043072893821280037010279"
	  ],
	  [
	   "1",
	   "0"
	  ]
	 ],
	 "vk_gamma_2": [
	  [
	   "10857046999023057135944570762232829481370756359578518086990519993285655852781",
	   "11559732032986387107991004021392285783925812861821192530917403151452391805634"
	  ],
	  [
	   "8495653923123431417604973247489272438418190587263600148770280649306958101930",
	   "4082367875863433681332203403145435568316851327593401208105741076214120093531"
	  ],
	  [
	   "1",
	   "0"
	  ]
	 ],
	 "vk_delta_2": [
	  [
	   "9578317963632610151336009162988348724787547079411333329677896001428855832312",
	   "15223652499722719159424906581777102078862523870106238832193552620426177289449"
	  ],
	  [
	   "7265010420518973797514673122071363300792836381472918485237957338548759328172",
	   "15818506560941235416309800918620469588826088456435875574829581240997757125029"
	  ],
	  [
	   "1",
	   "0"
	  ]
	 ],
	 "vk_alphabeta_12": [
	  [
	   [
	    "14988641425665988543173012437855962873430267927938843845212377695680893509401",
	    "13376631292518172229524588785085616596640262697110939214207125670596806668662"
	   ],
	   [
	    "9665928106521994323707670301003991839963339714237244404040672746749971183445",
	    "11286286613183756834700510175396685972593158891616055183077552693217821866140"
	   ],
	   [
	    "14670343343674699726672162746105329956494703426891625238828562002584076829768",
	    "5335774586431426730845318728369258274051480530378671404594262495169846336906"
	   ]
	  ],
	  [
	   [
	    "13064684412674695810467070467168442433339019821373530072208985706106784339796",
	    "496264313146617107248544685516502919968496068699196759329190902119056326572"
	   ],
	   [
	    "2674640337879171525175235435666044353562373445419785183123002628033923203032",
	    "19393605000452539039361872279996726097592730963031488252366290344458812187303"
	   ],
	   [
	    "17258633520774171265962566789183247690305589194147697336919680418587614956516",
	    "15787466130746444516485041757175598132724707998252298761321817658606865792978"
	   ]
	  ]
	 ],
	 "IC": [
	  [
	   "12446248444031568981644257616045744421897424289005791262520820980003833304613",
	   "14126708031549800150251786798334874245826413261481575346471080364230164514748",
	   "1"
	  ],
	  [
	   "11599899900117225048041999225445311506159975436943878453332146996355954055091",
	   "12297636032842741549067951675347041044599253527166300668918276287765908064748",
	   "1"
	  ],
	  [
	   "19735911389920508717813623267801440121340989591129667156007378533800446155427",
	   "6370726871639726261638177994806792690899743443170098478085371913895858297436",
	   "1"
	  ],
	  [
	   "3056357461368316928050448698580197612474935705336531040887295932203479094319",
	   "4578265088046347073940510401159988824211136564709579407923367900462670239568",
	   "1"
	  ],
	  [
	   "10395748184835372666023760228174670695333928071630341607951040865085882942077",
	   "9750990852540117403757726698930048971735350814370660404001667648661895269869",
	   "1"
	  ],
	  [
	   "1126132770607892831010140912602315025459598690608356545941023769519478104014",
	   "1866290431477058083436703633558469917331614919276756798997938992757656788463",
	   "1"
	  ]
	 ]
	}
	`
	vk0, err := snarkParsers.ParseVk([]byte(vkJSON))
	qt.Assert(t, err, qt.IsNil)
	app.ZkVks = append(app.ZkVks, vk0)

	processId := arbo.BigIntToBytes(32, big.NewInt(10))
	entityId := []byte("entityid-test")
	censusRootBI, ok := new(big.Int).SetString("13256983273841966279055596043431919350426357891097196583481278449962353221936", 10)
	qt.Assert(t, ok, qt.IsTrue)
	process := &models.Process{
		ProcessId: processId,
		EntityId:  entityId,
		EnvelopeType: &models.EnvelopeType{
			Anonymous: true,
		},
		Mode:       &models.ProcessMode{},
		Status:     models.ProcessStatus_READY,
		CensusRoot: arbo.BigIntToBytes(32, censusRootBI),
	}
	err = app.State.AddProcess(process)
	qt.Assert(t, err, qt.IsNil)

	protoProof := models.ProofZkSNARK{
		CircuitParametersIndex: 0,
		A: []string{
			"8565618924009803349128724448755637260817268535764018694748071697999960309465",
			"10567467520683290855894011826361531515330977110060690707312139021253814259201",
			"1",
		},
		B: []string{
			"7331705443978474465740013805022628883217016623626657489677999970845326148519",
			"14088543429308940796639598167648469598115265384006174847203062566695157397842",
			"21650416714230850553304309420788184918411764695672027474950575748521240000392",
			"9608621404453315322574104078089794462171122070923660024556835674016394593207",
			"1",
			"0",
		},
		C: []string{
			"2581164907110383930740960311274244222988702887156649148820057728817188544394",
			"4874727216192126962804861520752733436306034259929110689845718277073170774972",
			"1",
		},
		PublicInputs: []string{"512956825156072911942930000072467850503272879939251541441978430922425553120", "302689215824177652345211539748426020171", "205062086841587857568430695525160476881"},
	}

	vtx := &models.VoteEnvelope{
		ProcessId: processId,
		Proof: &models.Proof{
			Payload: &models.Proof_ZkSnark{
				ZkSnark: &protoProof,
			},
		},
	}
	signature := []byte{}
	txBytes := []byte{}
	txID := [32]byte{}
	commit := false

	_, err = app.VoteEnvelopeCheck(vtx, txBytes, signature, txID, commit)
	qt.Assert(t, err, qt.IsNil)
}
