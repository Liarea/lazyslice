// SPDX-License-Identifier: Apache-2.0

package mask

import (
	"sort"
	"strings"
)

// The embedded word lists. They are part of the mapping, because K_cat is
// derived from the category and the generator indexes into these lists: adding
// a word changes every masked value in the categories that draw from it, which
// is a release note and a "classification changed" line, not a tidy-up
// (ARCHITECTURE.md section 5 "Determinism scope"). They are ours rather than
// gofakeit's for exactly that reason (ADR-006 "Not dependencies").
//
// Everything here is ASCII and lower case. A generator applies whatever
// capitalisation the output shape wants, so the same list serves an email
// local part and a person's name.
const (
	givenWords = `alice amara amelia andre anna anton arjun asha aurora ayla bram bruno camila
carlos cato chloe clara cyrus dalia daniel dario dawit dilan dmitri eda eli elena elias elin
emeka emil emma enzo esme ewan farah felix fiona freya gabriel gita greta hana hassan hector
helena hugo ida idris ilya ines ira isaac ivan jamal jana jasper javier jonas jorge josef juno
kai kaito kamil karim kasia keiko kenji khalid kira klara lars laila leon lena liam lina livia
lucas luisa mads maja malik marek maria mateo maya mei milan mira mirza nadia nasir nina noor
nuru olga oliver omar oscar otto paulo petra pia priya quinn rafael rania ravi rhea rita rosa
ruben ruth sami sana sasha selma sergei simon sofia soren stefan suri tariq tessa theo tomas
tova uma vera viktor vlad wren yara yosef yuki zaid zara zoe`

	surnameWords = `abara adler ahmed alvarez andersen antonov arnaud bakker baros bauer bennett
berg bianchi blanco boateng borg bosch brandt cabrera campos castro cheng cohen conti cortez
costa dahl dalgaard delgado diallo dimas dubois duran egan engel eriksen esposito faber farrell
fischer fontaine fowler gallo garcia gomez greco gruber haas hansen hayes herrera hoffman horvat
ibrahim iversen jansen jelinek jensen kaminski kane keller khan klein koch kovac kruger lambert
larsen lawson leclerc lehman leroy lindqvist lopez lozano lund madsen mahmoud marino martel
mendes meyer mitchell moreau moretti morales mueller nakamura navarro nielsen novak nowak okafor
olsen ortiz osei palmer pavlov pereira petrov popescu porter ramos reyes richter rivera roche
romano rossi roux ruiz sadiq salazar sandu santos sato schmidt schulz serrano silva simic
sinclair soares sorensen stein suzuki tanaka tavares thomsen torres tran ueda vargas vega verner
vidal vogel wagner walsh weber wilson yamada yilmaz zhang ziegler`

	// roleGivenWords and roleFamilyWords are RoleGiven's and RoleFamily's own
	// vocabulary (T-0287, the fix round that followed it, and the review round
	// after that one). personNameMasker's RoleFull branch and emailMasker both
	// read givenWords/surnameWords above, unchanged from v0.1.0 (mask/CLAUDE.md's
	// Domain()/Determinism-scope rule: a shared list's Domain() and every value
	// it already masks are this module's public contract, and a role-only need
	// is not a reason to move either). RoleGiven and RoleFamily read these two
	// instead: synthetic, pronounceable, letters-only tokens built from
	// consonant-vowel syllables, at several times either shared list's size.
	//
	// The reason is the residual scan (ARCHITECTURE.md section 6 item 3), not
	// vocabulary novelty for its own sake. A first_name or last_name column
	// masked to a single word off a real-name list of a hundred-odd entries
	// has a real chance of matching some *other* row's real value in the same
	// column purely by the pigeonhole principle -- docs/media/first-run.gif's
	// own Pagila fixture hit this on the very demo T-0287 exists to fix, and
	// mask/CLAUDE.md's T-0287 section recorded the open question as T-0292.
	//
	// **These lists are filtered, not merely synthesised, and the filtering is
	// the actual control.** The first fix round's comment here claimed the two
	// lists were "never drawn from a real-name dictionary and disjoint from"
	// givenWords/surnameWords "by construction" -- true about the *generation*
	// process (a syllable grammar, not a lookup into any name list) and false
	// as a claim about the *result*: a syllable grammar can still land on a
	// string that happens to be somebody's real name, the way a random ASCII
	// string can still land on "example.com". A fix-round review measured it:
	// of the two lists' original 900 entries each, 34 were literal entries of
	// internal/textsig/names.txt (siri, nero, leni, sibel and thirty more), and
	// materially more again -- gale, sage, mari, boris, titus among them --
	// were ordinary common given names, surnames or English words the
	// dictionary does not carry at all. Every token now in roleGivenWords and
	// roleFamilyWords has been checked against three corpora and excluded if it
	// appeared in any of them: internal/textsig/names.txt (this project's own
	// multilingual name dictionary, ARCHITECTURE.md section 4 "Signals"), a
	// census-style corpus of common given names and surnames independent of
	// this project (the zxcvbn-data name lists: several thousand entries each,
	// ranked by real population frequency), and a general English dictionary
	// (macOS's /usr/share/dict/words, ~235,000 entries) -- the last of these
	// also catching the plainer failure of a role token that is not a name at
	// all but an ordinary recognisable word. That filtering is a one-time,
	// offline editorial step over the committed word lists, the same way
	// givenWords/surnameWords above were originally hand-curated: none of the
	// three corpora is a build or runtime dependency of this module. Five
	// tokens the review's own finding named directly (manu, levon, deron,
	// lorin, mati) were not caught by any of the three corpora above and were
	// removed by hand on the finding's word alone -- a reminder that "checked
	// against three corpora" is still short of "checked against every real
	// name", which is the whole of the residual risk two paragraphs down.
	// mask/role_test.go pins the specific tokens the review found colliding as
	// a permanent regression guard against a reintroduced overlap, alongside a
	// machine-checked assertion that neither role list intersects
	// givenWords/surnameWords. internal/classify (which imports both this
	// module and internal/textsig, where this module may import neither, per
	// mask/CLAUDE.md's "Never" list) carries the fuller cross-check against the
	// live, current name dictionary in TestRoleWordsExcludeCurrentNameDictionary.
	//
	// **This narrows the lists (900 to 780 given, 900 to 863 family) and is
	// stated as a mitigation, not a proof.** A masked value can still coincide
	// with a real production name the three corpora above do not carry --
	// exactly the same "known false-positive surface, not a leak" framing
	// mask/CLAUDE.md's other reserved ranges already carry (example.com,
	// 192.0.2.1), extended here to a filtered corpus rather than a registered
	// range because a person's name has no registered range to borrow. What
	// changed is that "not in the space of names or words this module could
	// check against" replaces the false "not in the space of names at all";
	// the residual risk T-0292 recorded is unchanged by this filtering and is
	// still open, because a materially larger admissible domain -- widening
	// the *shared* real-name lists, per T-0292's own text -- is the fix that
	// would actually move it, and that is additive to a different pair of
	// lists than these.
	roleGivenWords = `bado balo bede bege belu bemu biba bibe bidi bido bifa bifi bimo bimu biva boda boli bomu bovu
bude bufe bumo bumu bunu busi dabo dafi dasa davo dedu defa dege dero dibi dide dife diru doso
dosu dufo dulo dulu duno duvi fabe febi febo fefa fese fesi fiba fibi fida fimu fobi fodi fogi
fomi fonu fosi fubi fubu fudu fufi fula fule funo fusu gada gafi gafu gami gamu ganu gari gasa
gasu gavu geba gebe gebi gebu gefi gego gele geme gesi gevi gilu gimi gira gisa gisi gisu godo
godu gomo gono goru gosu guve labe labo lagu lano lanu lavu lefa lega lego lemi lemu linu liva
livo livu loba lodi lofe lofi lofo logu lole losi loso ludu lufi lule lumo lumu luva mabo madu
malu mamu maso mave mefa mefo meme memi memu meno mesu meva mido mifa mife milu mimo miru misa
mofe mofo mogi moli momi mosi movi mube mudu mufe muge muno muru muso muve muvo muvu nadi nafi
nafo nali namo namu nanu naso nebo negu neno nera neru nibo nifo nime niro niru nobo nodu nofa
nofi nole nonu nosi novu nudo nuge nule nume numu nuna nunu raba rade rali ranu rava rebi regi
reri rero reru rifa rilo rilu robu rogi rugi rulo rumo safu sebi sedu sefi segi sene seso seva
sevi sidu sino sive sivi sovo sube subi suge sule sumi suvu vadi vafo varu vebe vefu vemi venu
veso vesu vevu vifi vilu vime vinu vobi vofe vogu voli volo vome vora vori vubu vumo vumu vure
vuri vusi bagil balel bamol baror bavar belun belur bibur bidal biner bogin bomul bubur bumus
bunir burer busal busol busun dafas dalun danas dasis deder defor defur degir degon denel derel
dilir direl dogas dogol dunes durir fafil fanus feban feror fesor fidal fimis finur fival fodul
fogal forer fubin fudal fugun fusun fuvur gafal gedos gemon genur gibal girul gobir godur gofan
gomin gonen gosor gubel gudel gudos gulun gusos lafar lafun lasos lasur lebar leren lidir lilal
lirus lises lonas lonur losun lugar lugon lunin lusis melal memen memir menen menes meval mifes
migil mimil mirol mivar monin movul mubil mufil mulen muras muror murus muvir nabas naril nefos
nelar nenis nilon niron nirun nodan nonan nubis nubus nudir nulir nulor numos nusen nuves rabur
raler refes reger relul remun ribir rifer rigos rilon rimos rodir rosir rosun rovus rufal rugas
rugel rumil runul ruvos ruvur sabus safos sanos sefus selar sevir sibus sidal sinon sisis sodon
sogon somis sosal sovis sufal sufir sufis sulir sunel suril suvul suvus vafer vagar vaven vedar
veden veger vevas vifin vigal vigin vodin vogus volur vovar vulul vunun balevo balumi bameri
bavuri bebele bedomu begubo bigilo bilafo bisaro bogabi bosabi bubili bufila bufose bulome
bumani buruno dadovo dafame dagobe dagudi daledi dalogu davumo demave devodi didila didura
digaba dimomi dinefe dirovu disame dofano dolole dolufu donali donola dosuga dovobi dumala
duraso durufi dusilo fafavi fafedi fanudo fasona fefano fegemu feluse fenasa fesene fesifo
finama fivuro folelu fomede fonelu foreni fosiso fufedu fugove funeso furafo gadada gadiri
gafano gafile gafonu garasi garave gavube gebone gebuna gedeso gefufe gegoda gelofe geluse
gemugi gesibe gesiga gevele gibadi gibugo giluno godidi gofoni gogodi gomalu gonafa goreru
govero guburo gudiro gufava gulofu gumure gusefo guvosi guvusu lagufo lalilu larevi lenero
lifami linogi linulu lirela lomime loniso lovudi lubase lufobo mavobu menoba menola migovu
mobamo mogufu munome nabane nabunu naveme nenofe nenofo nerubi nifado nifuso nileba nimalo
ninese nisolo nofega nogusu nomoga norufa novimi nubabi nudamu nudosi nugeso numire radise
rafamu rafigo ranimu rasolo ravada refami regevi regidu renana rerudu rigebu rilovo risobu
rogolu roresu rorivi rovugu rudigu rufila rulumi rumomu ruruge sadabe sagore samafo samivi
sarema sebuni segene segovo senive silalo sivuro sogegi sonavo sosibi sosomo sovoma suboli
sudemu sunifi vadalo vafegi vagilu vagobe variru vasobi vemavo veregi vesube vesuvi viledu
vimebi vinaro visuvo vobena vofefe vomime vonadi vovase vubuda vugemo vugudu vuguge vuladu
vunoge babobis baderen barulur befiler benagos benumas benurul bigisin biledir birovel bolubun
budivun dadoser dadubal dafisal daguson daruges dasomun defivar denulur dibavin donofal donolur
dorenul dorenun dudusas fanavar falofis febalin febomel fegulis fibilos fibiran fofafur fugusar
fulifor ganisen gedebel gemavin geredun gidedol girudin goribol gorigun gulenir lagidan lasadir
lasegul leforol legimes lemavur lenabis lidebor ligilon lirafol lolanus mabadon mafuban magebel
magogol malufus medesis memalel meneful merafas mevesis mibirel midefer mobafur mobages momiron
nafolin namavur namisus nanisal narulun nefugol nesiles niberil nigomon nobosas nofolir nuferol
nugifis numogur numunal nunavis radogol rafegor ranibal raruvan rasevor revenun rinunes rofosun
rofubos rovegen rubisir rugumal rusasus segonor sesabun simodir sirusen sovaver sugomar valiros
vebanol vefubir vegemar vibegar videmas vinalis vovonol`

	roleFamilyWords = `bika bobe boru defu degi dilu dizu duni faju fili fisi fisu foji foki gava geke gojo gosa gozi
gubu gumu gupa jali jazi jefu jevo jipo jume jumu kedo kize kodo lajo lare lefu leju leli lemo
ligu lika lufo lupu mada maji mifo neku nivo nupi nuta nuze paja pami pasu pebi pefa pema pemo
pevu piba pogu ponu puba puko rapi raza rigi risi rito ruve saji sapu seju siba siso sovi tamu
tavi tefi tike tozi vaba vaze veji viza voku vuza zabo zaja zefi zevu zile zime zisi zuli babiz
badis bajos bakaz bamin bapoz batus bekas besoz bigiz bobin bokus bomoz borus bugin bujon busos
buvoz dabin dames datan dates deges dezon diman dojin dojun duzis duzos fanun fasun febas fedan
femen fepez feven fifiz fijen fikez fikis filan fufin gakez gapan gavan gedan gemin gerun getan
giluz gobin gonas gumiz gupen jasos jasus javus jekan jidon jogun jubis julos jutus kadus kajun
kakaz kaken kakez kapon katon kazen kelin kemaz kigaz kozaz kuzen lebin lepin lepis lerez libos
lijas lipis lirez loduz manaz mavin medun mejez mejoz memes mibis mimaz mimun miniz mizes mobaz
modes mokus motis mudos mugin mujes naduz nakuz naves nekoz nelez nepes netos neziz nojoz nomus
novoz nozun petaz peten petin piniz pitiz pizan pizin pobuz poluz potus povis pubun rafon rames
rapaz rases rediz redon refun rezis rirez rofan rofen rufun sekez sikun sisez sogiz sulos sumes
tavuz tazon tedaz tepon tetez tevez tijez tikus timoz togiz tosas tukon tuvin valuz venuz vibez
vidis vizas vogan vokoz vonon vumon vunen vutoz zaboz zagos zavaz zezas zikus zodun zojaz zonos
zumaz zupon zures baguth bajaov bajoik batath bazoov bebais bejaik bepiov bezaen bezith bigeen
bijaik bizeis bodufi bomoik bujiov buloth buneth busuvu butois dabuov dafeka daguth dajuov
daneen danumi dasigo debifu delien dezano dezozi dimelo disema disiis diviov doguov donuov
dopuik dovedu dubosu dujelo dujoen duleen duluen dupifi dupiov durais fabida fafesu famuik
fanuov fasiov febuis fejiis fekipa fekozu feleth fenaik ferudi fesiis fetith fevige fezien
fidien finuth fiseik fitusu fivuov fizith fofaen fofeov fokoth folith fomiis foriov fosuik
foveis fukadu fukoen fuluik funaov furoth fusien futeen gafoen gajaik gamube gasoth gatova
gatuba gatuis gebith gedeov gegiik gejais gemiov ginath ginien ginois gobeth gosaen gosais
gozath gozith gufiis gujoen guliik jadien jaguis jajiis jalaen jaziik jeguov jekuen jekuth
jeleov jenuen jepien jeputh jesimu jetuth jibeik jigeth jilaik jimiov jipiis jirone jodabu
jofien jogeth jogith joguov joloik joluen jopiov jufuis jusuov jutoth juzaov juzasu kamois
kapoma kasoik kedaen keketh kepiis kevoen kigoov kiniis kirith kobuov kofiov koliku koruth
kubajo kukoen kunuov kuvais laboso ladoen lanaov larien lelois lepiov lesemo lesiov levais
levuis lisien litedu livija livois lodath lojuis lomiis loreve lozode luboth lujaov mabuth
mafigi malien maritu masaik medego mefeik mejiov menoen mesaje mibais mifimi mileku mimoov
minois modiik mogiri mopeov motiik mubavu mubefi mudath mudeen mugith munagi muroje musuov
muvuth naliov naveik navoth nazoov nefuth nepeis nepese neroen neteen netoen nijiki nikiik
nilesu nimoen nogiik nolais noruen nuroth nutebu pabuis panith paputh peleik peluba peneov
pevuis pideis piduth pifeni pifoik pipuik piraik pobuik poludu pudinu pukutu pumoov puneis
radufu ragibi ravoov razais razuth rekeen remosi repien rereik resaik resois retien revuov
ridaik rigoth riluov rinaik rineli rizith rodiik rofais ropien rosaov rotaik rufaen rupais
sabuov sagoov sanoli sateis sefais sefath semeik sidath sidois simuov sinavu sinefo sopiov
soseis subuov sufeth sufetu sujiik suteth suzoen taguov tamuis tebaov tejeov teloik telozi
tepago tevien tevoov tezuik tidezi tileth tomeis tozuth tufoth tumeis tupois turuth vagaov
vajaga valeze vapien vasizi vegiis vegoik vekais vepipi vijeov vijuik vikaen vipais viteik
vizoov vofoov vokoov vosath voveen vubeov vujath vukuth vupoen vuruen zabith zagaov zajuis
zaraik zaziik zebien zejava zepivo zeruik zezais zijeis zimeis zireik zitoov zizoen zodoen
zogeis zojira zojois zoneik zonete zoteik zovuov zugeik zupeth babokaz bebalos bemafoz bolaviz
boputun davivan deponun devoloz dijepoz dikoman dorugaz dutumen felopes fisalos foromis fozofoz
fubiniz fujosoz gabemin galedus ganodin gemufun gerebis gigimoz gitolas gobijan jagolon jegetan
jinetiz jonigis josulus jukosaz kazepun kepobus konilen kozaliz kufopes kulekin lesirus letojun
ligosin lokedaz lubunon lumadan mefokiz miraras mivujan moparun munenon muveron naberiz nakefus
nasiles novakoz nurusuz pibafes pirutos pitunon potapaz pugevos pulanin purunoz ralakon ralizas
rapades rasokaz redabus redegen rekuroz relefoz rerusoz rikimun rotumun rozezun rubufon rupatiz
ruvovas sanavuz satabon sevugas sezajes sigupon sovukus todokas tolepez vagakus vasinan vavudos
vumojuz vuvusan zigipes zinatan zirovan zitokan bamisaov barazaik benofaik berisuen betimais
bezezeen bidiraen bijafith bipitais borogoov budofoen bupubith busameik dagosais damijiik
daputuen dibajuis digomoen dopetiov dumanoik dumubeik fajiteik fatojiik foloroov foluneen
fonateis fonijeov fujugeov fuzebuis gagajoth gajadeov ganatien gatamuth genokeik gifajuth
gigeguen gijufiis gipiluik gonodoth gumuneov guneteth gurasiis jagizaov jamedeov jegafuik
jejopois jeniteen jetipiik jonobaik junabeov jurakoth jusetaen kajepiik keludoik kenabaik
kidefoen kokenuen konajuov kubilaik kunaboth kupanaen lafuraik lapekath lavaleen lazukiov
lijiteth linuliik lizukuik lomakois lovereik lupigeth lusatiov mabototh megisiik mekuloen
mibesuen midezoen mipuzaen miteleik morisaen mumeguis munikath muzokien naberiis nagulois
nilejath nilonaov nomujoen nulobiis nuroniov pebosuth peneliov penevoen peratuik pigiseik
poduseen pojeputh pokevaen popuzien pufajiik pukebiis pupofeis puzinith ratiziov ratoviov
repikuen rigudais rikujath ripeputh ritifith rubasiov rufesois rurusoen sagabuth sejibuov
sidapiis sivezaov sizidois sokuboov suzedeov temokuth tofiloth vatozeis vebazeth vetiziov
vetosoth vibunith vobunoth vutafiis vutesuov zerinuen zetidoik zibanaik zikopeth zivofeov
zovupeth zuzeriik`

	streetWords = `alder ash aspen beech birch bramble briar cedar cherry chestnut clover cypress
dogwood elder elm fern fir ginkgo hazel heather hemlock hickory holly ivy juniper larch laurel
lilac linden magnolia maple mulberry oak olive orchard palm pine poplar primrose quince redwood
rowan sage sequoia spruce sumac sycamore tamarack teak thistle thorn tupelo walnut willow yew`

	suffixWords = `avenue close court crescent drive gardens grove lane mews place road row street
terrace walk way`

	fillerWords = `aggregate batch buffer cadence cluster column cycle dataset default delta entry
factor field filter format gradient header index input interval item label layer ledger level
limit margin marker matrix measure method metric module node offset option output packet panel
parcel pattern phase pointer policy prefix process profile queue range record region report
request result routine sample scalar schema scope sector segment sequence series session signal
slice socket source spectrum stack stage stream string subject summary symbol system table target
template thread token topic trace unit value vector vertex volume window`
)

// nanpAreaCodes are the North American area codes for which libphonenumber
// accepts +1 <area> 555 01NN, the range reserved for fiction. The list is
// embedded rather than probed at start-up on purpose: probing would make the
// masked value depend on the phonenumbers version, so a dependency bump would
// silently remap every phone column. TestNANPAreaCodesAreStillValid checks the
// list against the library instead, so a bump that invalidates one is loud.
const nanpAreaCodes = `201202203204205206207208209210212213214215216217218219220223` +
	`224225226227228229231234235236239240248249250251252253254256` +
	`257260262263267269270272276279281283289301302303304305306307` +
	`308309310312313314315316317318319320321323324325326327329330` +
	`331332334336337339340341343346347350351352353354360361363364` +
	`365367368369380382385386401402403404405406407408409410412413` +
	`414415416417418419423424425428430431432434435437438440442443` +
	`445447448450458463464468469470474475478479480484500501502503` +
	`504505506507508509510512513514515516517518519520521522525526` +
	`527528529530531532533534539540541544548551557559561562563564` +
	`566567570571572573574575577579580581582584585586587588600601` +
	`602603604605606607608609610612613614615616617618619620622623` +
	`626628629630631633636639640641645646647650651656657658659660` +
	`661662667669670671672678680681682683686689701702703704705706` +
	`707708709712713714715716717718719720724725726727728730731732` +
	`734737738740742743747748753754757760762763765769770771772773` +
	`774775778779780781782784785786787800801802803804805806807808` +
	`809810812813814815816817818819820821825826828829830831832833` +
	`835838839840843844845847848849850854855856857858859860862863` +
	`864865866867870872873876877878879888900901902903904905906907` +
	`908909910912913914915916917918919920925928929930931934936937` +
	`938939940941942943945947948949951952954956959970971972973975` +
	`978979980984985986989`

// maxFit is the largest length the fit table indexes. Every embedded word is
// far shorter; a budget above it is treated as unbounded.
const maxFit = 32

// wordList is an embedded list ordered by length, so the words that fit a
// budget are always a prefix of it and both the count and the choice are O(1).
type wordList struct {
	words []string
	fit   [maxFit + 1]int // fit[m] = how many words are at most m bytes long
}

func newWordList(s string) *wordList {
	w := &wordList{words: strings.Fields(s)}
	sort.Slice(w.words, func(i, j int) bool {
		if len(w.words[i]) != len(w.words[j]) {
			return len(w.words[i]) < len(w.words[j])
		}
		return w.words[i] < w.words[j]
	})
	for m := 0; m <= maxFit; m++ {
		n := 0
		for _, x := range w.words {
			if len(x) <= m {
				n++
			}
		}
		w.fit[m] = n
	}
	return w
}

// count is how many words fit a budget. A budget of 0 means unbounded and a
// negative budget means the caller already knows nothing fits.
func (w *wordList) count(budget int) int {
	if budget < 0 {
		return 0
	}
	if budget == 0 || budget > maxFit {
		return len(w.words)
	}
	return w.fit[budget]
}

// longest is the length of the longest word in the list.
func (w *wordList) longest() int { return len(w.words[len(w.words)-1]) }

// shortest is the length of the shortest word in the list.
func (w *wordList) shortest() int { return len(w.words[0]) }

var (
	givenNames      = newWordList(givenWords)
	surnames        = newWordList(surnameWords)
	roleGivenNames  = newWordList(roleGivenWords)
	roleFamilyNames = newWordList(roleFamilyWords)
	streets         = newWordList(streetWords)
	suffixes        = newWordList(suffixWords)
	fillers         = newWordList(fillerWords)
	areaCodes       = splitThrees(nanpAreaCodes)
	emailHosts      = []string{"example.com", "example.net", "example.org"}
	docPrefixes     = []string{"192.0.2.", "198.51.100.", "203.0.113."}
)

// RoleWords returns a copy of the word list personNameMasker draws from for
// role -- roleGivenNames for RoleGiven, roleFamilyNames for RoleFamily -- in
// the order Mask indexes them, or nil for RoleFull, which draws a pair
// rather than a single list. It exists so a caller with a larger name corpus
// of its own can audit the list for an entry that is also a real name,
// without this module reaching into that corpus itself: this module may
// import nothing under internal/ (mask/CLAUDE.md's "Never" list), so the
// cross-check against internal/textsig's own name dictionary has to run on
// the other side of that boundary, in internal/classify, which already
// imports both. It is not part of the masking algorithm §5 fixes -- nothing
// here reads it -- so exposing it is additive and changes no masked value.
func RoleWords(r Role) []string {
	var w *wordList
	switch r {
	case RoleGiven:
		w = roleGivenNames
	case RoleFamily:
		w = roleFamilyNames
	default:
		return nil
	}
	out := make([]string, len(w.words))
	copy(out, w.words)
	return out
}

func splitThrees(s string) []string {
	out := make([]string, 0, len(s)/3)
	for i := 0; i+3 <= len(s); i += 3 {
		out = append(out, s[i:i+3])
	}
	return out
}

// pairCount is how many (a, b) pairs fit budget bytes with a separator of
// sepLen between them. A budget of 0 means unbounded, a negative one nothing.
func pairCount(a, b *wordList, sepLen, budget int) int64 {
	if budget < 0 {
		return 0
	}
	if budget == 0 || budget >= a.longest()+sepLen+b.longest() {
		return int64(len(a.words)) * int64(len(b.words))
	}
	var total int64
	for _, x := range a.words {
		room := budget - sepLen - len(x)
		if room < b.shortest() {
			break // words are ordered by length, so nothing after x fits either
		}
		total += int64(b.count(room))
	}
	return total
}

// pairAt returns the n-th pair in the same order pairCount counts them. n must
// be in [0, pairCount).
func pairAt(a, b *wordList, sepLen, budget int, n int64) (string, string) {
	if budget == 0 || budget >= a.longest()+sepLen+b.longest() {
		wide := int64(len(b.words))
		return a.words[n/wide], b.words[n%wide]
	}
	for _, x := range a.words {
		room := budget - sepLen - len(x)
		if room < b.shortest() {
			break
		}
		k := int64(b.count(room))
		if n < k {
			return x, b.words[n]
		}
		n -= k
	}
	// Unreachable for an n below pairCount; the shortest pair is the honest
	// fallback rather than a panic in a masking path.
	return a.words[0], b.words[0]
}
